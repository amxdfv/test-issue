package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"main/database"
	natsLog "main/nats"
	"net/http"
	"net/url"
	"strconv"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

// RestHandler структура для обработчика запросов
type RestHandler struct {
	DataBase *sql.DB
	Redis    *redis.Client
	Nats     *nats.Conn
}

// PostBody тело входящего POST и PATCH запроса
type PostBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Priority    int    `json:"newPriority"`
}

// NewRestHandler получаем новый обработчик запросов
func NewRestHandler(db *sql.DB, rdb *redis.Client, nc *nats.Conn) RestHandler {
	return RestHandler{
		DataBase: db,
		Redis:    rdb,
		Nats:     nc,
	}
}

// GetHandler обрабочик get-запроса
func (rh RestHandler) GetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit, offset, err := getLimitAndOffset(r.URL.Query())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	payload, err := database.FindInCache(rh.Redis, limit, offset)
	if payload != nil {
		w.WriteHeader(200)
		w.Write(payload)
		return
	} else {
		log.Print(err) // продолжаем
	}

	payload, err = database.FindGoods(rh.DataBase, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}
	// пишем кеш
	err = database.PutInCache(rh.Redis, payload, limit, offset)
	if err != nil {
		log.Print(err)
	}

	w.WriteHeader(200)
	w.Write(payload)
}

// PostHandler обрабочик post-запроса
func (rh RestHandler) PostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	spID := r.URL.Query().Get("projectId")
	if spID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("projectId not provided"))
		return
	}
	pID, err := strconv.Atoi(spID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	jsonBody, err := readBody(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	if jsonBody.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("name not provided"))
		log.Print("name not provided")
		return
	}

	payload, logPayload, err := database.InsertGood(rh.DataBase, pID, jsonBody.Name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}
	err = database.InvalidateCache(rh.Redis)
	if err != nil {
		log.Print(err)
	}
	err = natsLog.SendLog(rh.Nats, logPayload)
	if err != nil {
		log.Print(err)
	}
	w.WriteHeader(200)
	w.Write(payload)
}

// DeleteHandler обрабочик delete-запроса
func (rh RestHandler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	values := r.URL.Query()
	ID, pID, err := getIDAndProjectID(values.Get("id"), values.Get("projectId"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	payload, logPayload, err := database.DeleteGood(rh.DataBase, ID, pID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			w.Write(payload)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}
	err = database.InvalidateCache(rh.Redis)
	if err != nil {
		log.Print(err)
	}
	err = natsLog.SendLog(rh.Nats, logPayload)
	if err != nil {
		log.Print(err)
	}
	w.WriteHeader(200)
	w.Write(payload)
}

// UpdateHandler обрабочик Update-запроса
func (rh RestHandler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	values := r.URL.Query()
	ID, pID, err := getIDAndProjectID(values.Get("id"), values.Get("projectId"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	jsonBody, err := readBody(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	if jsonBody.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("name not provided"))
		log.Print("name not provided")
		return
	}

	payload, logPayload, err := database.UpdateGood(rh.DataBase, ID, pID, jsonBody.Name, jsonBody.Description)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			w.Write(payload)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}
	err = database.InvalidateCache(rh.Redis)
	if err != nil {
		log.Print(err)
	}
	err = natsLog.SendLog(rh.Nats, logPayload)
	if err != nil {
		log.Print(err)
	}
	w.WriteHeader(200)
	w.Write(payload)
}

// ReprioritiizeHandler обрабочик Repreoritiize-запроса
func (rh RestHandler) ReprioritiizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	values := r.URL.Query()
	ID, pID, err := getIDAndProjectID(values.Get("id"), values.Get("projectId"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	jsonBody, err := readBody(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	if jsonBody.Priority == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("newPriority not provided"))
		log.Print("newPriority not provided")
		return
	}

	payload, logPayload, err := database.ReprioritiizeGood(rh.DataBase, ID, pID, jsonBody.Priority)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			w.Write(payload)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}
	err = database.InvalidateCache(rh.Redis)
	if err != nil {
		log.Print(err)
	}

	// пишем в лог
	for _, msg := range logPayload {
		out, err := json.Marshal(msg)
		if err != nil {
			log.Print(err)
			continue
		}
		err = natsLog.SendLog(rh.Nats, out)
		if err != nil {
			log.Print(err)
		}
	}
	w.WriteHeader(200)
	w.Write(payload)
}

// getLimitAndOffset получаем лимит и отступ для sql
func getLimitAndOffset(params url.Values) (limit, offset int, err error) {
	l := params.Get("limit")
	if l == "" {
		l = "10"
	}
	limit, err = strconv.Atoi(l)
	if err != nil {
		return 0, 0, err
	}

	o := params.Get("offset")
	if o == "" {
		o = "1"
	}
	offset, err = strconv.Atoi(o)
	if err != nil {
		return 0, 0, err
	}
	return limit, offset, err
}

// getIDAndProjectID получаем id и projectId
func getIDAndProjectID(sID, spID string) (ID, pID int, err error) {
	if sID == "" {
		err = errors.New("id not provided")
		return
	}
	if spID == "" {
		err = errors.New("projectId not provided")
		return
	}
	pID, err = strconv.Atoi(spID)
	if err != nil {
		return
	}
	ID, err = strconv.Atoi(sID)
	if err != nil {
		return
	}
	return
}

// readBody читаем тело запроса
func readBody(in io.ReadCloser) (jsonBody PostBody, err error) {
	body, err := io.ReadAll(in)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &jsonBody)
	if err != nil {
		return
	}
	return
}
