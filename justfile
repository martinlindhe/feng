# generate smoketest files. do this manually to update the references with known changes
gen_smoketest:
	go run cmd/smoketester/main.go ./smoketest/smoketest.yml

# verify current output vs the pre-generated smoketest files, in order to spot regressions
verify_smoketest:
	go test ./smoketest -run TestCompareWithReferenceParse
