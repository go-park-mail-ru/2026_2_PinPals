# Нормализация схемы БД

## БД состоит из следующих таблиц: 

    User - все данные о Учётной записи пользователя и его профиле
    Desk - все данные о тематической коллекции пинов, создаваемая пользователем.
    Pin - все данные о публикации изображения с названием и описанием,
    Commentary - все данные о комментарии и его содержании,
    Pin_like - все данные о том, кто поставил лайк на конкретный пин,
    Comment_like - все данные о том, кто поставил лайк на конкретный коммент,
    Subscription - данные о подписке и участниках этого взаимоотношения,
    Pin_desc_relation - все данные о том, к каким доскам привязан конкретный пин,
    Chat - все данные о чате и его участниках, 
    Chat_message - все данные о сообщениях в чатах, 
    Tag - все данные о теге, 
    Pin_Tag_relation - все данные о том, к каким пинам привязан конкретный тег
    User_session — данные о завершённых сессиях пользователя (активные сессии хранятся в Redis).


## Дополнительное хранилище: MinIO, Redis

Изображения загружаются в объектное хранилище MinIO. В бд же хранятся ссылки (например, `avatar_url`, `image_url`, `avatar_img_url`)

В Redis хранястя актуальные сессии пользователей и чатов. После выхода пользователя данные о его сессии передаются в user_sessions, а данные из  чатов в chat и chat_message

---

# Отношения и функциональные зависимости

## 1 Нормальная форма:

        User:
        {user_id}    -> name, user_tag, age, description, avatar_url, created_at, updated_at — первичный суррогатный ключ
        {user_tag}   -> user_id, name, age, description, avatar_url, created_at, updated_at — естественный ключ

    Desk:
        {desk_id} -> creator_id, name, description, avatar_img_url, entry_status, created_at, updated_at — первичный суррогатный ключ

    Pin:
        {pin_id}    -> creator_id, image_url, name, description, created_at, updated_at, deleted_at — первичный суррогатный ключ

    Commentary:
        {comment_id} -> author_id, post_id, body, created_at, updated_at, deleted_at — первичный суррогатный ключ

    Pin_like:
        {pin_like_id}       -> liker_id, post_id, created_at — первичный суррогатный ключ
        {liker_id, post_id} -> pin_like_id, created_at — составной потенциальный ключ (UNIQUE)

    Comment_like:
        {comm_like_id}          -> commenter_id, comm_id, created_at — первичный суррогатный ключ
        {commenter_id, comm_id} -> comm_like_id, created_at — составной потенциальный ключ (UNIQUE)

    Subscription:
        {subscription_id}              -> subscribee_id, subscriber_id, created_at — первичный суррогатный ключ
        {subscribee_id, subscriber_id} -> subscription_id, created_at — составной потенциальный ключ (UNIQUE)

    Pin_desk_relation:
        {desk_relation_id} -> desk_id, pin_id, created_at — первичный суррогатный ключ
        {desk_id, pin_id}  -> desk_relation_id, created_at — составной потенциальный ключ (UNIQUE)

    Chat:
        {chat_id}              -> user_a_id, user_b_id, name, created_at, updated_at — первичный суррогатный ключ
        {user_a_id, user_b_id} -> chat_id, name, created_at, updated_at — составной потенциальный ключ (UNIQUE)

    Chat_message:
        {message_id} -> chat_id, author_id, body, created_at, updated_at, related_to — первичный суррогатный ключ

    Tag:
        {tag_id} -> name, created_at — первичный суррогатный ключ
        {name}   -> tag_id, created_at — естественный ключ

    Pin_tag_relation:
        {tag_relation_id} -> tag_id, pin_id, created_at — первичный суррогатный ключ
        {tag_id, pin_id}  -> tag_relation_id, created_at — составной потенциальный ключ (UNIQUE)

    User_session:
        {session_id} -> user_id, session_start, session_end — первичный суррогатный ключ


## 2 Нормальная форма:

    1НФ выполняется, а также все первичные ключи: user_id, desk_id, pin_id, comment_id, pin_like_id, comm_like_id, subscription_id, desk_relation_id, chat_id, message_id, tag_id, tag_relation_id,session_id  - атомарны, не массивы, а значит частичная зависимость невозможна по определению, потому что ключ состоит из одного атрибута. Каждый неключевой атрибут зависит от ключа целиком — дробить его не на что. Значит, 2НФ выполняется автоматически. 

    Отношения с составным потенциальным ключом:во всех таблицах с потенциальным составнымключом(Pin_like, Comment_like, Subscription, Pin_desk_relation, Pin_tag_relation) единственный неключевой атрибут — created_at. Докажем, что нет зависимости от части этого ключа:

    Pin_like: created_at — это время конкретного лайка. Оно не определяется ни liker_id (один пользователь лайкает разные пины в разное время), ни post_id (один пин лайкают разные пользователи в разное время). Значит, created_at зависит от всей пары {liker_id, post_id}.

    Comment_like: Также, как и лайк пина, лайк комментария не зависит от commenter_id(один пользователь лайкает разные комментарии в разное время), comm_id(один комментарий лайкают разные пользователи в разное время). Значит, created_at зависит от всей пары {comm_id, commenter_id}

    Subscription: created_at не зависит от subscribee_id или subscriber_id по отдельности(поскольку много пользователей м=подписываются на множетсов пользователей в разное время). Значит, created_at зависит от всей пары {subscribee_id, subscriber_id}

    Pin_desk_relation: created_at не зависит от  desk_id или pin_id по отдельности(к множеству досок прикрепляют множество пинов в разное время). Значит, created_at зависит от всей пары {desk_id, pin_id}

    Pin_tag_relation: created_at не зависит от tag_id или pin_id по отдельности(к множество пинов прикрепляют множество тегов в разное время). Значит, created_at зависит от всей пары {tag_id, pin_id}

## 3 Нормальная форма:

    2 НФ выполняется. В полях: Desk, Pin, Commentary, Chat_message, User_session нет транзитивных зависимостей: ни один неключевой атрибут не определяет другой неключевой атрибут. поля зависят только от ключей, а не друг от друга. В таблицах Pin_like, Comment_like, Subscription, Pin_desk_relation, Chat, Pin_tag_relation ровно один неключевой атрибут, не являющийся частью составного ключа, значит, транзитивных зависимостей между неключивыми атрибутами быть не может.

    User:

        Нет транзитивных зависимостей: ни один неключевой атрибут не определяет другой неключевой атрибут, то есть неключевые поля не зависят друг от друга. А также оба потенциальных ключа определяют все остальные атрибуты

    Tag:

        Нет транзитивных зависимостей: ни один неключевой атрибут не определяет другой неключевой атрибут, то есть неключевые поля не зависят друг от друга. А также оба потенциальных ключа определяют все остальные атрибуты

## НФБК:

    Отношения с одним потенциальным ключом (Desk, Pin, Commentary, Chat, Chat_message, User_session): единственный детерминант — первичный ключ. Он является суперключом. Все нетривиальные функциональные зависимости имеют слева суперключ, а значит НФБК выполняется.

    Отношения с составным потенциальным ключом (Pin_like, Comment_like, Subscription, Pin_desk_relation, Pin_tag_relation, Chat): детерминанты — {суррогатный PK} и {составной CK}. Оба являются потенциальными ключами (а значит, суперключами). Все нетривиальные функциональные зависимости имеют слева суперключ, в том числе:
    {liker_id, post_id} <-> pin_like_id
    {commenter_id, comm_id} <-> comm_like_id
    {subscribee_id, subscriber_id} <-> subscription_id
    {desk_id, pin_id} <-> desk_relation_id
    {user_a_id, user_b_id} <-> chat_id
    {tag_id, pin_id}  <-> tag_relation_id

    User потенциальные ключи — {user_id}, {user_tag}. Оба детерминанта - суперключами. В том числе: user_id <-> user_tag

    Tag потенциальные ключи — {tag_id} и {name}. Оба детерминанта — суперключи. В том числе: tag_id <-> name
