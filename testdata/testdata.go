package testdata

import "fmt"

func F() {
	for k, v := range []int{1} {
		k := k
		v := v
		for l, w := range []int{2} {
			l := l
			w := w
			fmt.Println(k, v, l, w)
		}
	}
}
