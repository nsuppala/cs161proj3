// Main logic/functionality for the web application.
// This is where you need to implement your own server.
package main

// Reminder that you're not allowed to import anything that isn't part of the Go standard library.
// This includes golang.org/x/
import (
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	_ "io/ioutil"
	"net/http"
	_ "os"
	"path/filepath"
	_ "path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func processRegistration(response http.ResponseWriter, request *http.Request) {
	username := request.FormValue("username")
	password := request.FormValue("password")

	// Check if username already exists
	row := db.QueryRow("SELECT username FROM users WHERE username = ?", username)
	var savedUsername string
	err := row.Scan(&savedUsername)
	if err != sql.ErrNoRows {
		response.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(response, "username %s already exists", savedUsername)
		return
	}

	// Generate salt
	const saltSizeBytes = 16
	salt, err := randomByteString(saltSizeBytes)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	hashedPassword := hashPassword(password, salt)

	_, err = db.Exec("INSERT INTO users VALUES (NULL, ?, ?, ?)", username, hashedPassword, salt)

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Set a new session cookie
	initSession(response, username)

	// Redirect to next page
	http.Redirect(response, request, "/", http.StatusFound)
}

func processLoginAttempt(response http.ResponseWriter, request *http.Request) {
	// Retrieve submitted values
	username := request.FormValue("username")
	password := request.FormValue("password")

	row := db.QueryRow("SELECT password, salt FROM users WHERE username = ?", username)

	// Parse database response: check for no response or get values
	var encodedHash, encodedSalt string
	err := row.Scan(&encodedHash, &encodedSalt)
	if err == sql.ErrNoRows {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response, "unknown user")
		return
	} else if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Hash submitted password with salt to allow for comparison
	submittedPassword := hashPassword(password, encodedSalt)

	// Verify password
	if submittedPassword != encodedHash {
		fmt.Fprintf(response, "incorrect password")
		return
	}

	// Set a new session cookie
	initSession(response, username)

	// Redirect to next page
	http.Redirect(response, request, "/", http.StatusFound)
}

func processLogout(response http.ResponseWriter, request *http.Request) {
	// get the session token cookie
	cookie, err := request.Cookie("session_token")
	// empty assignment to suppress unused variable warning
	_, _ = cookie, err

	// get username of currently logged in user
	username := getUsernameFromCtx(request)
	// empty assignment to suppress unused variable warning
	_ = username

	//////////////////////////////////
	// BEGIN TASK 2: YOUR CODE HERE
	//////////////////////////////////

	// TODO: clear the session token cookie in the user's browser
	// HINT: to clear a cookie, set its MaxAge to -1
	cookie.MaxAge = -1
	http.SetCookie(response, cookie)

	// TODO: delete the session from the database
	_, err = db.Exec("DELETE FROM sessions WHERE username = ?", username)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
	}

	//////////////////////////////////
	// END TASK 2: YOUR CODE HERE
	//////////////////////////////////

	// redirect to the homepage
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func processUpload(response http.ResponseWriter, request *http.Request, username string) {

	//////////////////////////////////
	// BEGIN TASK 3: YOUR CODE HERE
	//////////////////////////////////

	// HINT: files should be stored in const filePath = "./files"
	// get file
	file, header, err := request.FormFile("file")
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
	}

	// extract file name and veirfy
	filename := header.Filename
	matched, _ := regexp.MatchString("^(?:[[:alnum:]]|[.]){1,50}$", filename)
	if !matched {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(response, "invalid file name")
	}

	// extract file contents
	filecontents, err := ioutil.ReadAll(file)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
	}
	filecontents = []byte(filecontents)

	// create file path
	filepath := filepath.Join("./files", filename)

	// write file to disk
	err = ioutil.WriteFile(filepath, filecontents, 0644)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
	}

	// update files database table
	_, err = db.Exec("INSERT INTO files (owner, username, filename, filepath) VALUES (?, ?, ?, ?)", username, username, filename, filepath)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
	}
	http.Redirect(response, request, "/list", http.StatusFound)

	//////////////////////////////////
	// END TASK 3: YOUR CODE HERE
	//////////////////////////////////
}

// fileInfo helps you pass information to the template
type fileInfo struct {
	Filename  string
	FileOwner string
	FilePath  string
}

func listFiles(response http.ResponseWriter, request *http.Request, username string) {
	files := make([]fileInfo, 0)

	//////////////////////////////////
	// BEGIN TASK 4: YOUR CODE HERE
	//////////////////////////////////

	// for each of the user's files, add a
	// corresponding fileInfo struct to the files slice.
	rows, err := db.Query("SELECT owner, filename, filepath FROM files WHERE username =?", username)

	if err != nil {
		log.Fatal(err)
	}

	var (
		owner, filename, filepath string
	)

	for rows.Next() {
		err = rows.Scan(&owner, &filename, &filepath)

		if err != nil {
			log.Fatal(err)
		}
		file := fileInfo{Filename: filename, FileOwner: owner, FilePath: filepath}
		files = append(files, file)
	}

	//////////////////////////////////
	// END TASK 4: YOUR CODE HERE
	//////////////////////////////////

	data := map[string]interface{}{
		"Username": username,
		"Files":    files,
	}

	tmpl, err := template.ParseFiles("templates/base.html", "templates/list.html")
	if err != nil {
		log.Error(err)
	}
	err = tmpl.Execute(response, data)
	if err != nil {
		log.Error(err)
	}
}

func getFile(response http.ResponseWriter, request *http.Request, username string) {
	fileString := strings.TrimPrefix(request.URL.Path, "/file/")

	_ = fileString

	//////////////////////////////////
	// BEGIN TASK 5: YOUR CODE HERE
	//////////////////////////////////
	// check to see if user is allowed to download
	rows, err := db.Query("SELECT filepath, filename FROM files WHERE username =?", username)

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var filepath string
	var filename string
	authorized := false

	for rows.Next() {
		err = rows.Scan(&filepath, &filename)

		if err != nil {
			log.Fatal(err)
		}

		if filepath == fileString {
			authorized = true
			break
		}
	}

	// Download file
	if authorized {
		setNameOfServedFile(response, filename)
		http.ServeFile(response, request, fileString)
	} else {
		response.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(response, "not authorized to download")
		return
	}

	//////////////////////////////////
	// END TASK 5: YOUR CODE HERE
	//////////////////////////////////
}

func setNameOfServedFile(response http.ResponseWriter, fileName string) {
	response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
}

func processShare(response http.ResponseWriter, request *http.Request, sender string) {
	recipient := request.FormValue("username")
	filename := request.FormValue("filename")
	_ = filename

	if sender == recipient {
		response.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(response, "can't share with yourself")
		return
	}

	//////////////////////////////////
	// BEGIN TASK 6: YOUR CODE HERE
	//////////////////////////////////

	// check to see if the sender is allowed to send
	rows, err := db.Query("SELECT filename, filepath FROM files WHERE owner =?", sender)

	if err != nil {
		log.Fatal(err)
	}

	var (
		file, filepath string
	)
	authorized := false

	for rows.Next() {
		err = rows.Scan(&file, &filepath)

		if err != nil {
			log.Fatal(err)
		}

		if file == filename {
			authorized = true
		}
	}

	// update files database table
	if authorized {
		_, err := db.Exec("INSERT INTO files (owner, username, filename, filepath) VALUES (?, ?, ?, ?)", sender, recipient, filename, filepath)
		if err != nil {
			fmt.Fprintf(response, err.Error())
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(response, "file shared")
	} else {
		response.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(response, "not authorized to share file")
		return
	}

	//////////////////////////////////
	// END TASK 6: YOUR CODE HERE
	//////////////////////////////////

}

// Initiate a new session for the given username
func initSession(response http.ResponseWriter, username string) {
	// Generate session token
	sessionToken, err := randomByteString(16)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	expires := time.Now().Add(sessionDuration)

	// Store session in database
	_, err = db.Exec("INSERT INTO sessions VALUES (NULL, ?, ?, ?)", username, sessionToken, expires.Unix())
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Set cookie with session data
	http.SetCookie(response, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  expires,
		SameSite: http.SameSiteStrictMode,
	})
}
