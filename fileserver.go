package main

import (
	"archive/zip"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

func (app *application) handleHome(w http.ResponseWriter, r *http.Request) {
	log.Printf("in handleHome -req path: %q", r.URL.Path)
	tmpl := app.templateCache["home.page.html"]
	files, err := listFiles(*targetDir)
	if err != nil {
		log.Println("error while listing targetDir:", err)
		http.Error(w, "internal server error while listing", http.StatusInternalServerError)
		return
	}
	filesPathes := make([]FileInfo, 0, len(files))
	for i, f := range files {
		fi, err := f.Info()
		if err != nil {
			log.Println("error while listing targetDir:", err)
			http.Error(w, "internal server error while getting file info", http.StatusInternalServerError)
		}
		createdAt := fi.ModTime()
		fPath := filepath.Join("download", f.Name())
		file := FileInfo{Fname: f.Name(), FPath: fPath, Index: i, CreatedAt: createdAt}
		filesPathes = append(filesPathes, file)
	}

	tmpl.Execute(w, filesPathes)
	// app.templateCache.ExecuteTemplate(w, "homepage.html", filesPathes)

}

func (app *application) handleServingFiles(w http.ResponseWriter, r *http.Request) {
	log.Println("handleServingFiles just got hit")
	log.Printf("path:%q\n", r.URL.Path)
	tmpl := app.templateCache["files.page.html"]

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/files/")
	sourcePath := filepath.Join(*sourceDir, trimmedPath)
	files, err := listFiles(sourcePath)
	if err != nil {
		var perr *fs.PathError
		if errors.As(err, &perr) {
			http.ServeFile(w, r, sourcePath) // if it's not a directory,just serve it
			return
		}
		log.Printf("error returned from listFiles(): %v\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	filesPathes := make([]FileInfo, 0, len(files))
	for i, f := range files {
		fPath := path.Join(r.URL.Path, f.Name())
		if f.IsDir() {
			file := FileInfo{Fname: f.Name(), FPath: fPath, IsDir: true, Index: i}
			filesPathes = append(filesPathes, file)
			continue
		}

		file := FileInfo{Fname: f.Name(), FPath: fPath, Index: i}
		filesPathes = append(filesPathes, file)
	}

	tmpl.Execute(w, filesPathes)
	// app.templateCache.ExecuteTemplate(w, "files.html", filesPathes)
}

func (app *application) makeZip(w http.ResponseWriter, r *http.Request) {
	log.Printf("inside makeZip handler - %q\n", r.URL.Path)

	val := r.FormValue("zip")
	log.Printf("form value is %q\n", val)
	file := strings.TrimPrefix(val, "/files/")
	log.Printf("%s is getting zipped...", file)
	baseName := filepath.Base(file)
	zipName := baseName + ".zip"
	dirPath := filepath.Dir(file)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipName))
	w.Header().Set("Content-Type", "application/octet-stream")
	err := zipIt(dirPath, baseName, zipName, w)
	if err != nil {
		log.Printf("returned error from zipIt: %v\n", err)
		http.Error(w, "internal server error: the file not zipped", http.StatusInternalServerError)
		return
	}

	log.Printf("%s is just zipped...", file)
}

func zipIt(dirPath, baseName, zipName string, w io.Writer) error {
	fPath := filepath.Join(*targetDir, zipName)
	tmpfile, err := os.Create(fPath)
	if err != nil {
		log.Printf("error after os.create: err:%v\n", err)
		return err
	}
	defer tmpfile.Close()

	mw := io.MultiWriter(w, tmpfile)
	zw := zip.NewWriter(mw)
	defer zw.Close()

	walkdir := filepath.Join(*sourceDir, dirPath, baseName)
	fmt.Printf("walkdir: %q\n", walkdir)
	err = filepath.Walk(walkdir, func(path string, info fs.FileInfo, err error) error {
		log.Printf("inside walkFunc -- walkdir: %q\n", walkdir)
		log.Printf("walking: %#v\n", path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			log.Printf("error while opening the file path: %q\n", path)
			return err
		}
		defer file.Close()
		newPath := strings.TrimPrefix(path, walkdir)
		log.Printf("newPath: %q\n", newPath)
		f, err := zw.Create(newPath)
		if err != nil {
			log.Printf("zw.Create(newPath) - error while creating the file path: %q\n", newPath)
			return err
		}
		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (app *application) handleDownload(w http.ResponseWriter, r *http.Request) {
	log.Println("r.url.path:", r.URL.Path)
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	log.Printf("filename: %q\n", filename)
	fPath := filepath.Join(*targetDir, filename)
	log.Printf("fPath: %q\n", fPath)
	http.ServeFile(w, r, fPath)
}

type application struct {
	templateCache map[string]*template.Template
}

// FileInfo holds the file infos
type FileInfo struct {
	IsDir     bool // true if its a directory
	FPath     string
	Fname     string
	Index     int
	CreatedAt time.Time
}

func (app *application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		app.handleHome(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/files/") && r.Method == "GET" {
		app.handleServingFiles(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/files/") && r.Method == "POST" {
		app.makeZip(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/download/") && r.Method == "GET" {
		app.handleDownload(w, r)
		return
	} else {
		http.NotFound(w, r)
	}
}

var sourceDir = flag.String("s", "", "source directory path")
var targetDir = flag.String("t", "", "source directory path")
var port = flag.String("p", ":8080", "port for listening")

//go:embed templates
var content embed.FS

func formatDate(t time.Time) string {
	return t.Format("02 Jan 2006 at 15:04")
}

func currentYear() string {
	return fmt.Sprint(time.Now().Year())
}

var functions = template.FuncMap{
	"formatDate":  formatDate,
	"currentYear": currentYear,
}

var users = map[string]string{
	"user": "112233",
}

func main() {
	flag.Parse()
	if *sourceDir == "" || *targetDir == "" {
		fmt.Println("supply source and target directory to run")
		flag.Usage()
		os.Exit(1)
	}

	templateCache, err := newTemplateCache("templates")
	if err != nil {
		log.Fatalln("error while making template cache: ", err)
	}

	app := &application{templateCache: templateCache}

	log.Printf("starting server on %s\n", *port)
	err = http.ListenAndServe(*port, loggingMiddleware(app))
	if err != nil {
		log.Fatalln(err)
	}
}
func loggingMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Add("WWW-Authenticate", `Basic realm="restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if !isAuthorized(username, password) {
			http.Error(w, "you are not allowed", http.StatusUnauthorized)
			return
		}
		log.Printf("user:%s - password:%s\n", username, password)
		next.ServeHTTP(w, r)
	}
}

func isAuthorized(username, password string) bool {
	pwd, ok := users[username]
	if !ok {
		return false
	}
	return pwd == password
}

func listFiles(path string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func newTemplateCache(dirname string) (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}
	pages, err := content.ReadDir(dirname)
	if err != nil {
		log.Printf("error while reading directory: %v\n", err)
		return nil, err
	}
	log.Printf("pages:%+v\n", pages)
	for _, p := range pages {
		name := p.Name()
		if !strings.HasSuffix(name, "page.html") {
			log.Printf("%s got jumped\n", name)
			continue
		}
		ts, err := template.New(name).Funcs(functions).ParseFS(content, dirname+"/"+name)
		if err != nil {
			log.Printf("error while parsing file:%s -- err:%v\n", name, err)
			return nil, err
		}
		ts, err = ts.ParseFS(content, dirname+"/"+"*partial.html")
		if err != nil {
			return nil, err
		}
		cache[name] = ts
	}

	return cache, nil
}
