package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"gopkg.in/gomail.v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/gorilla/mux"
)

var DB *gorm.DB

func InitDB() {
	var err error
	dsn := "user=our_user password=our_password dbname=certificate_db host=localhost port=5432 sslmode=disable"
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	DB.AutoMigrate(&Certificate{})
}

type Certificate struct {
	ID      int    `json:"id" gorm:"primarykey"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Owner   string `json:"owner"`
	Date    int    `json:"date"`
}

var certificates []Certificate
var nextID int = 1

func GetCertificateByID(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var certificate Certificate
	if err := DB.First(&certificate, id).Error; err != nil {
		http.Error(w, "Certificate not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(certificate)
}

func CreateCertificate(w http.ResponseWriter, r *http.Request) {
	var newCertificate Certificate

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&newCertificate); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Save the new certificate in the database
	if err := DB.Create(&newCertificate).Error; err != nil {
		http.Error(w, "Error creating certificate", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newCertificate)
}

func GetAllCertificates(w http.ResponseWriter, r *http.Request) {
	var certificates []Certificate
	if err := DB.Find(&certificates).Error; err != nil {
		http.Error(w, "Error fetching certificates", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(certificates)
}

func UpdateCertificate(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var updatedCertificate Certificate
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updatedCertificate); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var certificate Certificate
	if err := DB.First(&certificate, id).Error; err != nil {
		http.Error(w, "Certificate not found", http.StatusNotFound)
		return
	}

	// Update fields
	certificate.Name = updatedCertificate.Name
	certificate.Content = updatedCertificate.Content
	certificate.Owner = updatedCertificate.Owner
	certificate.Date = updatedCertificate.Date

	// Save updated certificate in the database
	if err := DB.Save(&certificate).Error; err != nil {
		http.Error(w, "Error updating certificate", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(certificate)
}

func SendCertificate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var request struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid email", http.StatusBadRequest)
		return
	}

	// Fetch certificate from DB
	var certificate Certificate
	if err := DB.First(&certificate, id).Error; err != nil {
		http.Error(w, "Certificate not found", http.StatusNotFound)
		return
	}

	// Set up the email message
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", "your_email@gmail.com")
	mailer.SetHeader("To", request.Email)
	mailer.SetHeader("Subject", "Your Certificate: "+certificate.Name)
	mailer.SetBody("text/plain", "Here is your certificate!")

	// Set up the SMTP server
	dialer := gomail.NewDialer("smtp.gmail.com", 587, "your_email@gmail.com", "your_email_password")
	if err := dialer.DialAndSend(mailer); err != nil {
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Certificate sent successfully!")
}
func SendBulkEmail(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Emails  []string `json:"emails"`
		Content string   `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Send emails to each recipient
	for _, email := range request.Emails {
		err := sendEmail(email, request.Content)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error sending email to %s: %v", email, err), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Bulk emails sent successfully!")
}

func sendEmail(to string, content string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", "your_email@gmail.com")
	mailer.SetHeader("To", to)
	mailer.SetHeader("Subject", "Certificate")
	mailer.SetBody("text/plain", content)

	dialer := gomail.NewDialer("smtp.gmail.com", 587, "your_email@gmail.com", "your_email_password")
	return dialer.DialAndSend(mailer)
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer your-secret-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Initialize the database
	InitDB()

	r := mux.NewRouter()

	// Define routes
	r.Handle("/certificates/{id}", AuthMiddleware(http.HandlerFunc(GetCertificateByID))).Methods("GET")
	r.Handle("/certificates", AuthMiddleware(http.HandlerFunc(CreateCertificate))).Methods("POST")
	r.Handle("/certificates", AuthMiddleware(http.HandlerFunc(GetAllCertificates))).Methods("GET")
	r.Handle("/certificates/{id}", AuthMiddleware(http.HandlerFunc(UpdateCertificate))).Methods("PUT")
	r.Handle("/send/{id}", AuthMiddleware(http.HandlerFunc(SendCertificate))).Methods("POST")
	r.Handle("/send_bulk", AuthMiddleware(http.HandlerFunc(SendBulkEmail))).Methods("POST")

	// Start the server
	log.Fatal(http.ListenAndServe(":8000", r))
}
