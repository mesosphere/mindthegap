INCLUDE_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(INCLUDE_DIR)make.mk
include $(INCLUDE_DIR)shell.mk
include $(INCLUDE_DIR)help.mk
include $(INCLUDE_DIR)repo.mk
include $(INCLUDE_DIR)platform.mk
include $(INCLUDE_DIR)tools.mk
include $(INCLUDE_DIR)pre-commit.mk
include $(INCLUDE_DIR)go.mk
include $(INCLUDE_DIR)goreleaser.mk
include $(INCLUDE_DIR)docker.mk
include $(INCLUDE_DIR)ci.mk
include $(INCLUDE_DIR)tag.mk
include $(INCLUDE_DIR)skopeo.mk
include $(INCLUDE_DIR)upx.mk
