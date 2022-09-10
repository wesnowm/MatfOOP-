.PHONY: all binary build clean install install-binary shell test-integration

export GO15VENDOREXPERIMENT=1

PREFIX ?= ${DESTDIR}/usr
INSTALLDIR=${PREFIX}/bin
MANINSTALLDIR=${PREFIX}/share/man
# TODO(runcom)
#BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := skopeo-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
# set env like gobuildtag?
DOCKER_FLAGS := docker run --rm -i #$(DOCKER_ENVS)
# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_FLAGS += -t
endif
DOCKER_RUN_DOCKER := $(DOCKER_FLAGS) "$(DOCKER_IMAGE)"

GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)

all: binary

binary:
	go build -ldflags "-X main.gitCommit=${GIT_COMMIT}" -o ${DEST}skopeo ./cmd/skopeo

build-container:
	docker build ${DOCKER_BUILD_ARGS} -t "$(DOCKER_IMAGE)" .

clean:
	rm -f skopeo

install: install-binary
	install -m 644 man1/skopeo.1 ${MANINSTALLDIR}/man1/
	# TODO(runcom)
	#install -m 644 completion/bash/skopeo ${BASHINSTALLDIR}/

install-binary:
	install -d -m 0755 ${INSTALLDIR}
	install -m 755 skopeo ${INSTALLDIR}


shell: build-container
	$(DOCKER_RUN_DOCKER) bash

check: validate test-unit test-integration

# The tests can run out of entropy and block in containers, so replace /dev/random.
test-integration: build-container
	$(DOCKER_RUN_DOCKER) bash -c 'rm -f /dev/random; ln -sf /dev/urandom /dev/random; hack/make.sh test-integration'

test-unit: build-container
	# Just call (make test unit-local) here instead of worrying about environment differences, e.g. GO15VENDOREXPERIMENT.
	$(DOCKER_RUN_DOCKER) make test-unit-local

validate: build-container
	$(DOCKER_RUN_DOCKER) hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

# This target is only intended for development, e.g. executing it from an IDE. Use (make test) for CI or pre-release testing. 
test-all-local: validate-local test-unit-local

validate-local:
	hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

test-unit-local:
	go test $$(go list -e ./... | grep -v '^github\.com/projectatomic/skopeo/\(integration\|vendor/.*\)$$')
