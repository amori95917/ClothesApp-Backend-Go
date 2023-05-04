package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"io"
	"time"

	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	_ "github.com/lib/pq"

	// "example.com/m/v2/models"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "123456"
	dbname   = "postgres"
)

var db *sql.DB

func Connect() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Println("Successfully connected!")

	// var user models.User
	// stmt, err := db.Prepare("INSERT INTO users (username, email, photo) VALUES ($1, $2, $3)")
	// if err != nil {
	// 		return
	// }
	// defer stmt.Close()

	// _, err = stmt.Exec("trendy", "trendy0413dev@gmail.com", "trendy.png")
	// if err != nil {
	// 		return
	// }
	// fmt.Println("Successfully inserted a user!")
}

// Credentials which stores google ids.
type Credentials struct {
	Cid     string `json:"cid"`
	Csecret string `json:"csecret"`
}

// User is a retrieved and authentiacted user.
type User struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Profile       string `json:"profile"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Gender        string `json:"gender"`
}

var cred Credentials
var conf *oauth2.Config
var state string
var store = sessions.NewCookieStore([]byte("secret"))

func init() {
	file, err := ioutil.ReadFile("./creds.json")
	if err != nil {
		log.Printf("File error: %v\n", err)
		os.Exit(1)
	}
	json.Unmarshal(file, &cred)

	conf = &oauth2.Config{
		ClientID:     cred.Cid,
		// ClientSecret: cred.Csecret,
		// RedirectURL:  "http://127.0.0.1:8080/redirect",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email", // You have to select your own scope from here -> https://developers.google.com/identity/protocols/googlescopes#google_sign-in
		},
		Endpoint: google.Endpoint,
	}
}


func googleAuthHandler(c *gin.Context) {
	log.Println("Token: ", string(c.Query("token")))
	tok := c.Query("token")

	// Create a new oauth2.Token object with the token string
	token := &oauth2.Token{AccessToken: tok}

	client := conf.Client(oauth2.NoContext, token)
	response, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
	}
	defer response.Body.Close()
	data, _ := ioutil.ReadAll(response.Body)
	log.Println("Email body: ", string(data))

	type UserInfo struct {
    Email string `json:"email"`
	}

	var userInfo UserInfo
	err = json.Unmarshal(data, &userInfo)
	if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
	}

	email := userInfo.Email
	log.Println("Email: ", email)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&count)
	if err != nil {
			// handle error
			
		log.Printf("File error: %v\n", err)
	}

	log.Println(count)
	session := sessions.Default(c)
	if count > 0 {
		log.Println("Exist")
		session.Set("authenticated", true)
		session.Save()
		c.JSON(200, gin.H{"status": "Exist"})
	} else {
		log.Println("New User")
		session.Set("email", email)
		session.Save()
		c.JSON(200, gin.H{"status": "New User"})
	}
}

func registerHandler(c *gin.Context) {
	session := sessions.Default(c)
	retrievedEmail := session.Get("email")
	
	log.Println(retrievedEmail)

	// Parse the multipart form data
	form, err := c.MultipartForm()
	if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
			return
	}

	// Get the username field
	username := form.Value["username"][0]

	log.Println(username)
	// Get the photo file
	photo, err := form.File["photo"][0].Open()
	if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get photo"})
			return
	}
	defer photo.Close()

	fileName := "file-" + time.Now().Format("20060102150405") + ".jpg"
	// Create a new file on disk
	file, err := os.Create(fileName)
	if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
			return
	}
	defer file.Close()

	// Copy the photo to the file on disk
	_, err = io.Copy(file, photo)
	if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save photo"})
			return
	}

	// Save the username and file path to the database
	// ...
	
	stmt, err := db.Prepare("INSERT INTO users (username, email, photo) VALUES ($1, $2, $3)")
	if err != nil {
			c.JSON(500, gin.H{"error": "Failed to prepare SQL statement"})
			return
	}
	defer stmt.Close()

	_, err = stmt.Exec(username, retrievedEmail, fileName)
	if err != nil {
			c.JSON(500, gin.H{"error": "Failed to insert data into database"})
			return
	}
	
	c.JSON(200, gin.H{"success": "Registration Complete"})
}

func main() {

	Connect()

	router := gin.Default()
	router.Use(sessions.Sessions("goquestsession", store))
	
	router.GET("/google-auth", googleAuthHandler)
	router.POST("/register", registerHandler)
	router.Run(":8080")
}
