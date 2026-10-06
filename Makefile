ARG ?=
TIME ?= 30

measure:
	LZ77_CORPUS=$(ARG) go test -run TestMeasure -v -timeout $(TIME)m
