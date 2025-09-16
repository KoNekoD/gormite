git_tag_increment_minor_version:
	@TAG_ON_HEAD_COMMIT=$$(git tag --points-at HEAD); \
	if [ -n "$$TAG_ON_HEAD_COMMIT" ]; \
	then \
			echo "Tag '$$TAG_ON_HEAD_COMMIT' already exists on that commit. Skipping tagging."; \
	else \
			echo "Getting last tag..."; \
			LAST_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
			echo "Last tag: $$LAST_TAG"; \
			MAJOR=$$(echo $$LAST_TAG | cut -d. -f1); \
			MINOR=$$(echo $$LAST_TAG | cut -d. -f2); \
			PATCH=$$(echo $$LAST_TAG | cut -d. -f3); \
			NEW_PATCH=$$((PATCH + 1)); NEW_TAG="$${MAJOR}.$${MINOR}.$${NEW_PATCH}"; \
			echo "New tag: $$NEW_TAG"; \
			git tag "$$NEW_TAG"; \
			git push origin "$$NEW_TAG"; \
			echo "Tag '$$NEW_TAG' successfully created and pushed."; \
	fi

init_docs:
	make down
	make up_with_build
	docker exec -it docs-web yarn install

up: 
	docker compose up -d

up_with_build:
	docker compose up -d --build

down:
	docker compose down --remove-orphans