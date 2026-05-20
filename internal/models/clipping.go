package models

import (
	"database/sql"
	"errors"
	"time"
)

type Clipping struct {
	ID      string
	Status  string
	Format  string
	Created time.Time
}

type ClippingModel struct {
	DB *sql.DB
}

var ErrNotFound = errors.New("not found")

func (m *ClippingModel) Get(id string) (*Clipping, error) {
	query := `SELECT id, status, format, created
	FROM clippings
	WHERE id = $1`

	var c Clipping
	err := m.DB.QueryRow(query, id).Scan(&c.ID, &c.Status, &c.Format, &c.Created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &c, nil
}

func (m *ClippingModel) Insert(id string, format string) (*Clipping, error) {
	query := `INSERT INTO clippings (id, format)
	VALUES ($1, $2)
	RETURNING *`

	var clipping Clipping
	err := m.DB.QueryRow(query, id, format).Scan(&clipping.ID, &clipping.Status, &clipping.Format, &clipping.Created)
	if err != nil {
		return nil, err
	}

	return &clipping, nil
}

func (m *ClippingModel) Update(id string, status string) error {
	query := `UPDATE clippings
	SET status = $1
	WHERE id = $2`

	_, err := m.DB.Exec(query, status, id)
	if err != nil {
		return err
	}

	return nil
}
