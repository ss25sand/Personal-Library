package main

import (
	// "fmt"

	"html/template"
	"net/http"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"

	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/url"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

type LoginPage struct {
	Error string
}

type User struct {
	Username string `db:"username"`
	Secret   []byte `db:"secret"`
}

type Book struct {
	PK             int    `db:"pk"`
	Title          string `db:"title"`
	Author         string `db:"author"`
	Classification string `db:"classification"`
	User           string `db:"user"`
}

type GlobalPageItems struct {
	Books []Book
	User  string
}

type SearchResult struct {
	Title  string `xml:"title,attr"`
	Author string `xml:"author,attr"`
	Year   string `xml:"hyr,attr"`
	ID     string `xml:"owi,attr"`
}

var db *sql.DB

func verifyDatabase(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if err := db.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next(w, r)
}

var (
	key   = []byte("super-secret-key")
	store = sessions.NewCookieStore(key)
)

func verifyUser(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	if r.URL.Path == "/login" {
		next(w, r)
		return
	}

	session, _ := store.Get(r, "authentication")
	if session.Values["User"] != nil {
		var user User
		row, _ := db.Query("select username, secret from users where username = ?", session.Values["User"].(string))
		for row.Next() {
			row.Scan(&user.Username, &user.Secret)
		}
		if user.Username != "" {
			next(w, r)
			return
		}
	}
	http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
}

func main() {
	db, _ = sql.Open("sqlite3", "dev.db")

	mux := http.NewServeMux()

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "authentication")
		var p LoginPage
		if r.FormValue("register") != "" {
			secret, _ := bcrypt.GenerateFromPassword([]byte(r.FormValue("password")), bcrypt.DefaultCost)
			user := User{r.FormValue("username"), secret}
			_, err := db.Exec("insert into users (username, secret) values (?,?)", user.Username, user.Secret)
			if err != nil {
				p.Error = err.Error()
			} else {
				session.Values["User"] = user.Username
				session.Save(r, w)
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		} else if r.FormValue("login") != "" {
			var user User
			row, err := db.Query("select username, secret from users where username = ?", r.FormValue("username"))
			for row.Next() {
				row.Scan(&user.Username, &user.Secret)
			}
			if err != nil {
				p.Error = err.Error()
			} else if user.Username == "" && user.Secret == nil {
				p.Error = "No such user found with Username: " + r.FormValue("username")
			} else {
				err = bcrypt.CompareHashAndPassword(user.Secret, []byte(r.FormValue("password")))
				if err != nil {
					p.Error = err.Error()
				} else {
					session.Values["User"] = user.Username
					session.Save(r, w)
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}
			}
		}

		templates := template.Must(template.ParseFiles("templates/login.html")) // parses html file
		err := templates.ExecuteTemplate(w, "login.html", p)                    // write the html to response writer and passes object to index (can only pass one object)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError) // print the error
		}
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "authentication")
		session.Values["User"] = nil
		session.Save(r, w)
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "authentication")
		gpi := GlobalPageItems{Books: []Book{}, User: session.Values["User"].(string)} // closes the database
		rows, _ := db.Query("select pk, title, author, classification, user from books where user = ?", session.Values["User"].(string))
		for rows.Next() {
			var b Book
			rows.Scan(&b.PK, &b.Title, &b.Author, &b.Classification, &b.User)
			gpi.Books = append(gpi.Books, b)
		}

		templates := template.Must(template.ParseFiles("templates/index.html")) // parses html file
		err := templates.ExecuteTemplate(w, "index.html", gpi)                  // write the html to response writer and passes object to index (can only pass one object)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError) // print the error
		}
	})

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var results []SearchResult
		var err error

		if results, err = search(r.FormValue("search")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		encoder := json.NewEncoder(w)
		if err := encoder.Encode(results); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/books/add", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "authentication")

		var book ClassifyBookResponse
		var err error

		if book, err = find(r.FormValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		result, err := db.Exec("insert into books (pk, title, author, id, classification, user) values (?, ?, ?, ?, ?, ?)",
			nil, book.BookData.Title, book.BookData.Author, book.BookData.ID, book.Classification.MostPopular, session.Values["User"].(string))

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		pk, _ := result.LastInsertId()
		b := Book{
			PK:             int(pk),
			Title:          book.BookData.Title,
			Author:         book.BookData.Author,
			Classification: book.Classification.MostPopular,
			User:           session.Values["User"].(string),
		}

		err = json.NewEncoder(w).Encode(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/books/delete", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "authentication")
		_, err := db.Exec("delete from books where pk = ? and user = ?", r.FormValue("pk"), session.Values["User"].(string))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK) // sends 200 status code
	})

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(verifyDatabase))
	n.Use(negroni.HandlerFunc(verifyUser))
	n.UseHandler(mux)
	n.Run(":8080")
}

type ClassifySearchResponse struct {
	Results []SearchResult `xml:"works>work"`
}

type ClassifyBookResponse struct {
	BookData struct {
		Title  string `xml:"title,attr"`
		Author string `xml:"author,attr"`
		ID     string `xml:"owi,attr"`
	} `xml:"work"`
	Classification struct {
		MostPopular string `xml:"sfa,attr"`
	} `xml:"recommendations>ddc>mostPopular"`
}

func find(id string) (ClassifyBookResponse, error) {
	var c ClassifyBookResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&owi=" + url.QueryEscape(id))

	if err != nil {
		return ClassifyBookResponse{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c, err
}

func search(query string) ([]SearchResult, error) {
	var c ClassifySearchResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&title=" + url.QueryEscape(query))

	if err != nil {
		return []SearchResult{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c.Results, err
}

func classifyAPI(url string) ([]byte, error) {
	var resp *http.Response
	var err error

	if resp, err = http.Get(url); err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
