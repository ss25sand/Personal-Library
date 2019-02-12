# Personal-Library

## Description
Website that allows users to register and sign in (via a password hashed authentication system) to view there personal library and add new books to their book collection via a third party api called OCLC Classify.

## Brief Implementation Details
Website is implemented using a HTML, CSS, and JavaScript frontend and GO backend that includes a powerful http router called gorilla mux, different web middleware like negroni for logging purposes, a password hashing library called bcrypt, and an api that makes data queries to a SQLite database.

## Getting Started

### Prerequisites and Dependencies
```
Visit https://golang.org/dl/ and download Go Tools for the correct Operating System and preform any necessary configuration.
View the main.go file to see all the dependencies that need to be downloaded and download using go get [github links] command:
  - go get "github.com/mattn/go-sqlite3"
  - go get "github.com/codegangsta/negroni"
  - go get "github.com/gorilla/sessions"
  - go get "golang.org/x/crypto/bcrypt"
```

### Usage
After downloading all the dependencies, enter the ```go run main.go``` command in the terminal to start the server and visit http://localhost:8080 to view the website.
