# Copyright 2021 Mesosphere, Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: tag
tag:
ifndef NEW_GIT_TAG
	$(error Please specify git tag to create via NEW_GIT_TAG env var or make variable)
endif
	$(foreach module,\
		$(dir $(GO_SUBMODULES_NO_TOOLS)),\
		git tag -s "$(module)$(NEW_GIT_TAG)" -a -m "$(module)$(NEW_GIT_TAG)";\
	)
	git tag -s "$(NEW_GIT_TAG)" -a -m "$(NEW_GIT_TAG)"
