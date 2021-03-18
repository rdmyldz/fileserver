package main

import (
	"fmt"
	"path"
	"testing"
)

type deneme struct {
}

func TestDeneme(t *testing.T) {
	files, _ := listFiles("assets")
	f0 := files[0]
	fmt.Printf("files: %+v\n", f0)
	f0info, _ := f0.Info()
	fmt.Printf("f info:%+v\n", f0info.Name())
}

func Test_listFiles(t *testing.T) {
	t.Run("root directory", func(t *testing.T) {
		dirname := "."
		files, err := listFiles(".")
		// t.Errorf("err: %v -- files:%v\n", f, err)
		if err != nil {
			t.Errorf("err:%v", err)
		}
		for i, f := range files {
			// fi, _ := f.Info()

			// fmt.Printf("file info: %s\n", fi)
			if f.IsDir() {
				p := path.Join(dirname, f.Name())
				fmt.Printf("file%d:%v\n", i, p+"/")
				continue
			}
			p := path.Join(dirname, f.Name())
			fmt.Printf("file%d:%v\n", i, p)
		}
	})

	t.Run("assets directory", func(t *testing.T) {
		dirname := "assets"
		files, err := listFiles(dirname)
		// t.Errorf("err: %v -- files:%v\n", f, err)
		if err != nil {
			t.Errorf("err:%v", err)
		}
		for i, f := range files {
			if f.IsDir() {
				p := path.Join(dirname, f.Name())
				fmt.Printf("file%d:%v\n", i, p+"/")
				continue
			}
			p := path.Join(dirname, f.Name())
			fmt.Printf("file%d:%v\n", i, p)
		}

	})
	t.Run("assets/deneme1 directory", func(t *testing.T) {
		dirname := "assets/deneme1"
		files, err := listFiles(dirname)
		// t.Errorf("err: %v -- files:%v\n", f, err)
		if err != nil {
			t.Errorf("err:%v", err)
		}
		for i, f := range files {
			if f.IsDir() {
				p := path.Join(dirname, f.Name())
				fmt.Printf("file%d:%v\n", i, p+"/")
				continue
			}
			p := path.Join(dirname, f.Name())
			fmt.Printf("file%d:%v\n", i, p)
		}

	})
}

func Test_makeZip(t *testing.T) {
	zipIt("deneme1")
}
