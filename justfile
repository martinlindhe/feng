gen_smoketest:
	go run cmd/smoketester/main.go ./smoketest/smoketest.yml

verify_smoketest:
	go test ./... -run TestCompareWithReferenceParses
