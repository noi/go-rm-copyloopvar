package main

import (
	"fmt"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

func main() {
	if err := exec(os.Args[1]); err != nil {
		log.Fatalf("ERROR: %+v\n", err)
	}
}

func exec(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if fixed, err := fix(path, info.Mode().Perm()); err != nil {
			return err
		} else if fixed {
			fmt.Println("fixed:", path)
		}
		return nil
	})
}

func fix(path string, mode fs.FileMode) (bool, error) {
	src, err := os.Open(path)

	if err != nil {
		return false, err
	}
	f, err := decorator.Parse(src)
	if err != nil {
		return false, err
	}
	if err := src.Close(); err != nil {
		return false, err
	}

	fixed := false
	dst.Inspect(f, func(n dst.Node) bool {
		rangeStmt, ok := n.(*dst.RangeStmt)
		if !ok {
			return true
		}
		rangeStmtKey, ok := rangeStmt.Key.(*dst.Ident)
		if !ok {
			return true
		}
		rangeStmtValue, ok := rangeStmt.Value.(*dst.Ident)
		if !ok {
			return true
		}

		list := make([]dst.Stmt, 0, len(rangeStmt.Body.List))
		for _, s := range rangeStmt.Body.List {
			if isCopyLoopVar(s, rangeStmtKey.Name, rangeStmtValue.Name) {
				fixed = true
				continue
			}
			list = append(list, s)
		}
		rangeStmt.Body.List = list
		return true
	})

	if !fixed {
		return false, nil
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	if err := decorator.Fprint(tmp, f); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return false, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return false, err
	}
	return true, nil
}

func isCopyLoopVar(s dst.Stmt, kName, vName string) bool {
	assignStmt, ok := s.(*dst.AssignStmt)
	if !ok {
		return false
	}
	if assignStmt.Tok != token.DEFINE || len(assignStmt.Lhs) != 1 || len(assignStmt.Rhs) != 1 {
		return false
	}
	l, ok := assignStmt.Lhs[0].(*dst.Ident)
	if !ok {
		return false
	}
	r, ok := assignStmt.Rhs[0].(*dst.Ident)
	if !ok {
		return false
	}
	return l.Name == r.Name && (l.Name == kName || l.Name == vName)
}
