.PHONY: build install help clean

# Binary name
BINARY_NAME := kubectl-weka

# Get git information
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Check if tag is on current HEAD
TAG_ON_HEAD := $(shell git describe --exact-match --tags 2>/dev/null)

# Check if working directory is dirty (has uncommitted changes)
IS_DIRTY := $(shell git status --porcelain 2>/dev/null | wc -l)

# Calculate version based on whether tag is on HEAD
ifeq ($(TAG_ON_HEAD),)
  # Tag is NOT on current HEAD
  # Get number of commits since tag
  GIT_COMMITS := $(shell git rev-list --count $(GIT_TAG)..HEAD 2>/dev/null || echo "0")

  # Add "dirty" if there are uncommitted changes
  ifeq ($(IS_DIRTY),0)
    VERSION := $(GIT_TAG)-$(GIT_COMMITS)-$(GIT_COMMIT)
  else
    VERSION := $(GIT_TAG)-$(GIT_COMMITS)-$(GIT_COMMIT)-dirty
  endif
else
  # Tag IS on current HEAD - use tag as-is
  ifeq ($(IS_DIRTY),0)
    VERSION := $(TAG_ON_HEAD)
  else
    VERSION := $(TAG_ON_HEAD)-$(GIT_COMMIT)-dirty
  endif
endif

# ldflags for build
LDFLAGS := -ldflags="-X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)"

help:
	@echo "kubectl-weka Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build       Build binary in current directory"
	@echo "  make install     Install binary to GOPATH/bin"
	@echo "  make clean       Remove built binary"
	@echo "  make help        Show this help message"
	@echo ""
	@echo "Build Information:"
	@echo "  Version: $(VERSION)"
	@echo "  Commit:  $(GIT_COMMIT)"
	@echo "  Date:    $(BUILD_DATE)"
	@echo ""

build: .git-info
	@echo "Building $(BINARY_NAME) $(VERSION)"
	@echo "  Commit: $(GIT_COMMIT)"
	@echo "  Date:   $(BUILD_DATE)"
	go build $(LDFLAGS) -o $(BINARY_NAME) .

install: .git-info
	@echo "Installing $(BINARY_NAME) $(VERSION)"
	@echo "  Commit: $(GIT_COMMIT)"
	@echo "  Date:   $(BUILD_DATE)"
	go install $(LDFLAGS) .

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	@echo "Done"

.PHONY: .git-info
.git-info:
	@echo "Git Information:"
	@echo "  Latest Tag:     $(GIT_TAG)"
	@echo "  Tag on HEAD:    $(TAG_ON_HEAD)"
	@echo "  Working Dir:    $(if $(filter-out 0,$(IS_DIRTY)),dirty,clean)"
	@echo "  Version:        $(VERSION)"
	@echo "  Commit:         $(GIT_COMMIT)"



