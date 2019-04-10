test:
	mkdir -p tmp
	go run cmd/generate_test_model_columns.go 
	go test ./...

clean:
	rm tmp/*