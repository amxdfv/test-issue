package main

import (
	"log"
	"main/database"
	"main/handler"
	"net/http"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
		return
	}

	db, err := database.GetDatabase(cfg.Postgres)
	if err != nil {
		log.Fatal(err)
		return
	}

	r := handler.NewRestHandler(db)
	http.HandleFunc("/good", r.GetHandler)
	http.ListenAndServe(":8080", nil)
}
