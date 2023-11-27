package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"net/http"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"github.com/miekg/dns"
)

// DNSRecord represents a DNS record in the SQLite database
type DNSRecord struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

var db *sql.DB

func initDatabase() {
	var err error
	db, err = sql.Open("sqlite3", "./dns.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func addDNSRecord(name, recordType, value string) (int64, error) {
	result, err := db.Exec("INSERT INTO records (name, type, value) VALUES (?, ?, ?)", name, recordType, value)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	fmt.Printf("record created: %s", name)
	return id, nil
}

func updateDNSRecord(id int, record DNSRecord) error {
	_, err := db.Exec("UPDATE records SET name=?, type=?, value=? WHERE id=?", record.Name, record.Type, record.Value, id)
	fmt.Sprintf("updated record: %d", id)
	return err
}

func deleteDNSRecord(id int) error {
	_, err := db.Exec("DELETE FROM records WHERE id=?", id)
	fmt.Sprintf("deleted record: %d", id)
	return err
}

func queryDNS(name, recordType string) ([]DNSRecord, error) {
	rows, err := db.Query("SELECT * FROM records WHERE name=? AND type=?", name, recordType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DNSRecord
	for rows.Next() {
		var record DNSRecord
		err := rows.Scan(&record.ID, &record.Name, &record.Type, &record.Value)
		if err != nil {
			return nil, err
		}
		fmt.Printf("record read: %s", record.Name)
		records = append(records, record)
	}

	return records, nil
}
/*
func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		records, err := queryDNS(q.Name, dns.TypeToString[q.Qtype])
		if err != nil {
			log.Printf("Error querying DNS records: %v", err)
			continue
		}

		for _, record := range records {
			rr := new(dns.TXT)
			rr.Hdr = dns.RR_Header{Name: q.Name, Rrtype: dns.StringToType[record.Type], Class: dns.ClassINET, Ttl: 3600}
			rr.Txt = []string{record.Value}
			msg.Answer = append(msg.Answer, rr)
		}
	}

	w.WriteMsg(&msg)
}
*/

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
    fmt.Println("Processing DNS request")

    msg := dns.Msg{}
    msg.SetReply(r)
    msg.Authoritative = true

    for _, q := range r.Question {
        // Remove trailing dot, if present, from the domain name
        domainName := strings.TrimRight(q.Name, ".")

        fmt.Printf("Query: %s %s\n", domainName, dns.TypeToString[q.Qtype])

        records, err := queryDNS(domainName, dns.TypeToString[q.Qtype])
        if err != nil {
            log.Printf("Error querying DNS records: %v", err)
            continue
        }

        for _, record := range records {
            fmt.Printf("Record: %s %s %s\n", record.Name, record.Type, record.Value)

            switch q.Qtype {
            case dns.TypeA:
                rr := &dns.A{
                    Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600},
                    A:   net.ParseIP(record.Value),
                }
                msg.Answer = append(msg.Answer, rr)
                fmt.Printf("A record added: %s\n", domainName)
            case dns.TypeTXT:
                rr := &dns.TXT{
                    Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 3600},
                    Txt: []string{record.Value},
                }
                msg.Answer = append(msg.Answer, rr)
                fmt.Printf("TXT record added: %s\n", domainName)
            // Add more cases for other record types as needed
            }
        }
    }

    fmt.Println("Sending DNS response")
    if err := w.WriteMsg(&msg); err != nil {
        log.Printf("Error sending DNS response: %v", err)
    }
}


func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var record DNSRecord
		err := json.NewDecoder(r.Body).Decode(&record)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error decoding JSON: %v", err), http.StatusBadRequest)
			return
		}

		id, err := addDNSRecord(record.Name, record.Type, record.Value)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error adding DNS record: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int64{"id": id})
	case http.MethodGet:
		name := r.URL.Query().Get("name")
		recordType := r.URL.Query().Get("type")

		records, err := queryDNS(name, recordType)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error querying DNS records: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(records)
	case http.MethodPut:
		var record DNSRecord
		err := json.NewDecoder(r.Body).Decode(&record)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error decoding JSON: %v", err), http.StatusBadRequest)
			return
		}

		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		err = updateDNSRecord(id, record)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error updating DNS record: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		err = deleteDNSRecord(id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error deleting DNS record: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
    initDatabase()

    dns.HandleFunc(".", handleDNSRequest)
    server := &dns.Server{Addr: ":5300", Net: "udp", Handler: dns.DefaultServeMux}

    go func() {
        http.HandleFunc("/api/records", handleAPIRequest)
        log.Fatal(http.ListenAndServe(":8080", nil))
    }()

    log.Fatal(server.ListenAndServe())
}
