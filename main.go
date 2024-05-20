package main

import (
	"fmt"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
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
		var loopVarNames []string
		var body *dst.BlockStmt
		switch n := n.(type) {
		case *dst.RangeStmt:
			body = n.Body
			if n, ok := n.Key.(*dst.Ident); ok {
				loopVarNames = append(loopVarNames, n.Name)
			}
			if n, ok := n.Value.(*dst.Ident); ok {
				loopVarNames = append(loopVarNames, n.Name)
			}
		case *dst.ForStmt:
			body = n.Body
			if n, ok := n.Init.(*dst.AssignStmt); ok {
				for _, n := range n.Lhs {
					if n, ok := n.(*dst.Ident); ok {
						loopVarNames = append(loopVarNames, n.Name)
					}
				}
			}
		default:
			return true
		}
		if len(loopVarNames) == 0 {
			return true
		}

		list := make([]dst.Stmt, 0, len(body.List))
		for _, s := range body.List {
			if isCopyLoopVar(s, loopVarNames) {
				fixed = true
				continue
			}
			list = append(list, s)
		}
		body.List = list
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

func isCopyLoopVar(s dst.Stmt, loopVarNames []string) bool {
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
	return l.Name == r.Name && slices.Contains(loopVarNames, l.Name)
}
