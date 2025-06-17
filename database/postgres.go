package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	natsLog "main/nats"
	"time"

	_ "github.com/lib/pq"
)

// PostgresConfig конфиг для postgres
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// GoodsResponse структура ответа для get-запроса
type GoodsResponse struct {
	Meta  Meta   `json:"meta"`
	Goods []Good `json:"goods"`
}

// Meta метаданные для ответа
type Meta struct {
	Total   int `json:"total"`
	Removed int `json:"removed"`
	Limit   int `json:"limit"`
	Offset  int `json:"offset"`
}

// Good структура товара
type Good struct {
	ID          int        `json:"id"`
	ProjectID   int        `json:"projectId,omitempty"`
	Name        string     `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Priority    int        `json:"priority,omitempty"`
	Removed     bool       `json:"removed,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
}

// ReprioritiizeResponse структура ответа для изменения приоритета
type ReprioritiizeResponse struct {
	Priorities []Good `json:"priorities"`
}

// GetDatabase получаем коннекшн с базой
func GetDatabase(cfg PostgresConfig) (*sql.DB, error) {
	postgresqlDbInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
	return sql.Open("postgres", postgresqlDbInfo)
}

// FindGoods ищем товары
func FindGoods(db *sql.DB, limit, offset int) (payload json.RawMessage, err error) {
	rows, err := db.Query("select * from test_issue.goods order by id limit $1 offset $2", limit, offset)
	if err != nil {
		return nil, err
	}
	rowTotal, err := db.Query("select count(*) from test_issue.goods")
	if err != nil {
		return nil, err
	}
	rowRemoved, err := db.Query("select count(*) from test_issue.goods where removed = true")
	if err != nil {
		return nil, err
	}
	defer func() {
		rows.Close()
		rowTotal.Close()
		rowRemoved.Close()
	}()

	goods := make([]Good, 0, limit)
	for rows.Next() {
		good := Good{}
		err = rows.Scan(&good.ID, &good.ProjectID, &good.Name, &good.Description, &good.Priority, &good.Removed, &good.CreatedAt)
		if err != nil {
			return nil, err
		}
		goods = append(goods, good)
	}

	total := 0
	rowTotal.Next()
	err = rowTotal.Scan(&total)
	if err != nil {
		return nil, err
	}

	removed := 0
	rowRemoved.Next()
	err = rowRemoved.Scan(&removed)
	if err != nil {
		return nil, err
	}

	goodsResponse := GoodsResponse{
		Meta: Meta{
			Total:   total,
			Removed: removed,
			Limit:   limit,
			Offset:  offset,
		},
		Goods: goods,
	}

	out, err := json.Marshal(goodsResponse)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// InsertGood добавляем товар
func InsertGood(db *sql.DB, pID int, name string) (payload, logPayload json.RawMessage, err error) {
	row := db.QueryRow("insert into test_issue.goods (project_id, name) values($1, $2) returning *", pID, name)
	good := Good{}
	err = row.Scan(&good.ID, &good.ProjectID, &good.Name, &good.Description, &good.Priority, &good.Removed, &good.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	payload, err = json.Marshal(good)
	if err != nil {
		return nil, nil, err
	}
	logPayload, err = json.Marshal(natsLog.LogMessage{
		ID:          good.ID,
		ProjectID:   pID,
		Name:        good.Name,
		Description: good.Description,
		Priority:    good.Priority,
		Removed:     good.Removed,
		EventTime:   time.Now(),
	})
	if err != nil {
		return nil, nil, err
	}
	return payload, logPayload, nil
}

// DeleteGood помечаем товар удаленным
func DeleteGood(db *sql.DB, ID, pID int) (payload, logPayload json.RawMessage, err error) {
	res, err := db.Exec("update test_issue.goods set removed = true where id = $1 and project_id = $2", ID, pID)
	if err != nil {
		return nil, nil, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, nil, err
	}
	if rows == 0 {
		return []byte(notFoundMessage), nil, ErrNotFound
	}

	payload, err = json.Marshal(Good{
		ID:        ID,
		ProjectID: pID,
		Removed:   true,
	})
	if err != nil {
		return nil, nil, err
	}
	logPayload, err = json.Marshal(natsLog.LogMessage{
		ID:        ID,
		ProjectID: pID,
		Removed:   true,
		EventTime: time.Now(),
	})
	if err != nil {
		return nil, nil, err
	}
	return payload, logPayload, nil
}

// UpdateGood обновляем товар
func UpdateGood(db *sql.DB, ID, pID int, name, description string) (payload, logPayload json.RawMessage, err error) {
	desc := ""
	if description != "" {
		desc = ", description = $4"
	}
	statement := fmt.Sprintf("UPDATE test_issue.goods SET name = $3 %s WHERE id = $1 and project_id = $2 returning *;", desc)

	tx, err := db.Begin()
	if err != nil {
		return nil, nil, err
	}
	stmt, err := tx.Prepare(`SELECT * FROM test_issue.goods WHERE id = $1 and project_id = $2 FOR UPDATE;`)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	res, err := stmt.Exec(ID, pID)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	if rows == 0 {
		tx.Rollback()
		return []byte(notFoundMessage), nil, ErrNotFound
	}

	stmt, err = tx.Prepare(statement)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	updated := stmt.QueryRow(ID, pID, name, description)
	good := Good{}
	err = updated.Scan(&good.ID, &good.ProjectID, &good.Name, &good.Description, &good.Priority, &good.Removed, &good.CreatedAt)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, nil, err
	}

	payload, err = json.Marshal(good)
	if err != nil {
		return nil, nil, err
	}
	logPayload, err = json.Marshal(natsLog.LogMessage{
		ID:          good.ID,
		ProjectID:   pID,
		Name:        good.Name,
		Description: good.Description,
		Priority:    good.Priority,
		Removed:     good.Removed,
		EventTime:   time.Now(),
	})
	if err != nil {
		return nil, nil, err
	}
	return payload, logPayload, nil
}

// ReprioritiizeGood меняем приоритет у товара
func ReprioritiizeGood(db *sql.DB, ID, pID, priority int) (payload json.RawMessage, logPayload []natsLog.LogMessage, err error) {
	goods := []Good{}
	tx, err := db.Begin()
	if err != nil {
		return nil, nil, err
	}
	// в два подхода блокируем нужные записи в таблице
	stmt, err := tx.Prepare(`SELECT * FROM test_issue.goods WHERE id = $1 and project_id = $2 FOR UPDATE;`)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	res, err := stmt.Exec(ID, pID)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	rowsA, err := res.RowsAffected()
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	if rowsA == 0 {
		tx.Rollback()
		return []byte(notFoundMessage), nil, ErrNotFound
	}
	stmt, err = tx.Prepare(`SELECT * FROM test_issue.goods WHERE priority >= $1 FOR UPDATE;`)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	_, err = stmt.Exec(priority)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	stmt, err = tx.Prepare(`UPDATE test_issue.goods SET priority = priority+1 WHERE priority >= $1 returning id, priority;`)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	rows, err := stmt.Query(priority)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	for rows.Next() {
		good := Good{}
		err = rows.Scan(&good.ID, &good.Priority)
		if err != nil {
			return nil, nil, err
		}
		goods = append(goods, good)
	}

	stmt, err = tx.Prepare(`UPDATE test_issue.goods SET priority = $3 WHERE id = $1 and project_id = $2 returning id, priority;`)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	row := stmt.QueryRow(ID, pID, priority)
	good := Good{}
	err = row.Scan(&good.ID, &good.Priority)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	goods = append(goods, good)
	err = tx.Commit()
	if err != nil {
		return nil, nil, err
	}

	payload, err = json.Marshal(ReprioritiizeResponse{Priorities: goods})
	if err != nil {
		return nil, nil, err
	}
	for _, g := range goods {
		logPayload = append(logPayload, natsLog.LogMessage{
			ID:        g.ID,
			Priority:  g.Priority,
			EventTime: time.Now(),
		})
	}
	return payload, logPayload, nil
}

// notFoundMessage сообщение если товар не найден
var notFoundMessage = `"code": 3 "message": "errors.common.notFound" "details": {}`

// ErrNotFound сообщение если товар не найден
var ErrNotFound = errors.New("good not found")
