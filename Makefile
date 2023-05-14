GO      = /usr/bin/go
INSTALLPATH ?= /usr/bin
SERVICEPATH ?= $(HOME)/.config/systemd/user
CONFIGPATH ?= $(HOME)/.config/mouseless
service = mouseless.service
config = config.yaml
example_config = ./example_configs/config_example1.yaml
binary = mouseless

clean:
	-rm --force $(binary)

debug:
	@echo "# Stopping the service"
	-systemctl --user stop $(service)
	@echo ""

	@echo "# Run $(binary)"
	@echo "# config path: $(CONFIGPATH)/$(config)"
	$(GO) run . --config $(CONFIGPATH)/$(config) --debug
	@echo "################"

install:
	@echo "# Stopping and disabling the service"
	-systemctl --user disable --now $(service)
	@echo ""

	@echo "# Copying application to $(INSTALLPATH)"
	@echo "# This action requires sudo."
	@echo 'go build -ldflags="-s -w"'
	$(GO) build -ldflags="-s -w" .
	sudo cp --force $(binary) $(INSTALLPATH)
	sudo chmod u+s $(INSTALLPATH)/$(binary)
	@echo ""

	@echo "# Copying service file to $(SERVICEPATH)"
	mkdir --parents $(SERVICEPATH)
	cp --force $(service) $(SERVICEPATH)
	@echo ""

	@echo "# Copying default configuration file to $(CONFIGPATH)/$(config)"
	mkdir --parents $(CONFIGPATH)
	-cp --no-clobber $(example_config) $(CONFIGPATH)/$(config)
	@echo ""

	@echo "# Enabling and starting the service"
	systemctl --user daemon-reload
	systemctl --user enable --now $(service)

uninstall:
	@echo "# Stopping and disabling the service"
	-systemctl --user disable --now $(service)
	systemctl --user daemon-reload
	@echo ""

	@echo "# Removing service file from $(SERVICEPATH)"
	-rm $(SERVICEPATH)/$(service)
	-rm --dir $(SERVICEPATH)
	@echo ""

	@echo "# Removing application from $(INSTALLPATH)"
	@echo "# This action requires sudo."
	-sudo rm $(INSTALLPATH)/$(binary)
	@echo ""
