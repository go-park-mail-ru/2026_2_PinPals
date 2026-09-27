erDiagram
	direction TB
	USER {
		bigint user_id PK ""  
		text name  ""  
		text user_tag  ""  
		int age  ""  
		text description  ""  
		text avatar_url  ""  
		timestamptz created_at  ""  
		timestamptz last_updated_at  ""  
	}

	PIN {
		bigint pin_id PK ""  
		bigint creator_id FK ""  
		text image_url  ""  
		text name  ""  
		text description  ""  
		timestamptz created_at  ""  
		timestamptz last_updated_at  ""  
		timestamptz deleted_at  ""  
	}

	COMMENTARY {
		bigint comment_id PK ""  
		bigint author_id FK ""  
		bigint post_id FK ""  
		text body  ""  
		timestamptz created_at  ""  
		timestamptz last_updated_at  ""  
		timestamptz deleted_at  ""  
	}

	PIN_LIKE {
		bigint pin_like_id PK ""  
		bigint liker_id FK ""  
		bigint post_id FK ""  
		timestamptz created_at  ""  
	}

	COMMENT_LIKE {
		bigint comm_like_id PK ""  
		bigint commenter_id FK ""  
		bigint comm_id FK ""  
		timestamptz created_at  ""  
	}

	SUBSCRIPTION {
		bigint subscription_id PK ""  
		bigint subscribee_id FK ""  
		bigint subscriber_id FK ""  
		timestamptz created_at  ""  
	}

	CHAT {
		bigint chat_id PK ""  
		bigint user_a_id FK ""  
		bigint user_b_id FK ""  
		text name  ""  
		timestamptz created_at  ""  
		timestamptz updated_at  ""  
	}

	CHAT_MESSAGE {
		bigint message_id PK ""  
		bigint chat_id FK ""  
		bigint author_id FK ""  
		text body  ""  
		timestamptz created_at  ""  
		timestamptz updated_at  ""  
		bigint related_to FK ""  
	}

	PIN_DESK_RELATION {
		bigint desk_relation_id PK ""  
		bigint desk_id FK ""  
		bigint pin_id FK ""  
		timestamptz created_at  ""  
	}

	PIN_TAG_RELATION {
		bigint tag_relation_id PK ""  
		bigint tag_id FK ""  
		bigint pin_id FK ""  
	}

	DESK {
		bigint desk_id PK ""  
		bigint creator_id FK ""  
		text name  ""  
		text description  ""  
		text avatar_img_url  ""  
		smallint entry_status  ""  
		timestamptz created_at  ""  
		timestamptz last_updated_at  ""  
	}

	TAG {
		bigint tag_id PK ""  
		text name  ""  
		timestamptz created_at  ""  
	}

	MEDIA {
		bigint media_id PK ""  
		file filename  ""  
	}

	USER||--o{PIN:"creates"
	USER||--o{COMMENTARY:"writes"
	USER||--o{PIN_LIKE:"likes"
	USER||--o{COMMENT_LIKE:"likes"
	USER||--o{SUBSCRIPTION:"subscriber"
	USER||--o{SUBSCRIPTION:"subscribee"
	USER||--o{CHAT:"participant_a"
	USER||--o{CHAT:"participant_b"
	USER||--o{CHAT_MESSAGE:"author"
	PIN||--o{COMMENTARY:"has"
	PIN||--o{PIN_LIKE:"has"
	PIN||--o{PIN_DESK_RELATION:"belongs_to"
	PIN||--o{PIN_TAG_RELATION:"has"
	COMMENTARY||--o{COMMENT_LIKE:"has"
	DESK||--o{PIN_DESK_RELATION:"contains"
	CHAT||--o{CHAT_MESSAGE:"contains"
	CHAT_MESSAGE||--o{CHAT_MESSAGE:"reply_to"
	TAG||--o{PIN_TAG_RELATION:"tagged"