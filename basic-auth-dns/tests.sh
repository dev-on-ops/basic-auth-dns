#!/bin/bash

curl -X POST -H "Content-Type: application/json" -d '{"name":"example.com", "type":"A", "value":"192.168.1.1"}' http://localhost:8080/api/records

curl -X GET "http://localhost:8080/api/records?name=example.com&type=A"

curl -X PUT -H "Content-Type: application/json" -d '{"name":"example.com", "type":"A", "value":"192.168.1.2"}' http://localhost:8080/api/records?id=<record_id>

curl -X DELETE "http://localhost:8080/api/records?id=<record_id>"

