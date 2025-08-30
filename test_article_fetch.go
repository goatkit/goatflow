package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
	
	_ "github.com/lib/pq"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func main() {
	// Connect to database
	db, err := sql.Open("postgres", "postgres://gotrs_user:yggRU2-EjelkldX0M5EDBe_u@postgres:5432/gotrs?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	ticketID := "7"
	fmt.Printf("Testing article fetch for ticket %s\n", ticketID)
	
	// Run the exact query from the handler
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT a.id, a.article_sender_type_id, 
			   COALESCE(ast.name, 'Unknown') as sender_type,
			   COALESCE(adm.a_from, 'System') as from_addr,
			   COALESCE(adm.a_to, '') as to_addr,
			   COALESCE(adm.a_subject, 'Note') as subject,
			   COALESCE(adm.a_body, '') as body,
			   a.create_time,
			   a.is_visible_for_customer
		FROM article a
		LEFT JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE a.ticket_id = $1
		ORDER BY a.create_time DESC
	`, ticketID)
	
	if err != nil {
		log.Fatalf("Error fetching articles: %v", err)
	}
	defer rows.Close()
	
	count := 0
	for rows.Next() {
		var article struct {
			ID         int
			SenderTypeID int
			SenderType string
			From       string
			To         string
			Subject    string
			Body       string
			CreateTime time.Time
			IsVisible  bool
		}
		
		if err := rows.Scan(&article.ID, &article.SenderTypeID, &article.SenderType,
			&article.From, &article.To, &article.Subject, &article.Body, 
			&article.CreateTime, &article.IsVisible); err != nil {
			log.Printf("Error scanning article: %v", err)
			continue
		}
		
		count++
		fmt.Printf("\nArticle %d:\n", count)
		fmt.Printf("  ID: %d\n", article.ID)
		fmt.Printf("  Subject: %s\n", article.Subject)
		fmt.Printf("  From: %s\n", article.From)
		fmt.Printf("  Body (first 100 chars): %.100s\n", article.Body)
		fmt.Printf("  Create Time: %s\n", article.CreateTime.Format("2006-01-02 15:04:05"))
	}
	
	fmt.Printf("\nTotal articles found: %d\n", count)
}