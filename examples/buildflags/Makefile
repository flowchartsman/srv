SERVICE_VERSION := 0.0.1-alpha

LDFLAGS="-X andy.dev/srv/buildinfo.Version=$(SERVICE_VERSION)"

.PHONY: run
run:
	@$(eval TMPFILE := $(shell mktemp))
	@go build -ldflags ${LDFLAGS} -o $(TMPFILE)
	@echo "* Build flag values will appear in logs."
	@$(TMPFILE) --log-format text
	@echo
	@echo "* They will also appear in --version"
	@$(TMPFILE) --version
	@rm $(TMPFILE)
