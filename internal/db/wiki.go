package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type WikiDocument struct {
	ID       int
	URL      string
	Title    string
	FilePath string
}

func SearchWiki(db *sql.DB, queryEmbedding []float32, limit int) ([]WikiDocument, error) {
	var strVals []string
	for _, v := range queryEmbedding {
		strVals = append(strVals, fmt.Sprintf("%f", v))
	}
	vecStr := "[" + strings.Join(strVals, ",") + "]"

	rows, err := db.Query(`
		SELECT id, url, title, file_path 
		FROM wiki_documents 
		ORDER BY embedding <-> $1 
		LIMIT $2
	`, vecStr, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []WikiDocument
	for rows.Next() {
		var doc WikiDocument
		if err := rows.Scan(&doc.ID, &doc.URL, &doc.Title, &doc.FilePath); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
