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

## Дополнительное хранилище: MinIO

Изображения+ загружаются в объектное хранилище MinIO. В бд же хранятся ссылки (например, `avatar_url`, `image_url`, `avatar_img_url`)

---

# Отношения и функциональные зависимости

## 1 Нормальная форма:

    User:

        {user_id} -> name, user_tag, age, description, avatar_url, created_at, last_updated_at - первичный сурогатный ключ
        {user_tag} -> user_id, name, age, description, avatar_url, created_at, last_updated_at - естественный ключ

    Desk:

        {desk_id} -> creator_id, name, description, avatar_img_url, entry_status, created_at, last_updated_at - первичный сургатный ключ

    Pin:

        {pin_id} -> creator_id, image_url, name, description, created_at, last_updated_at, deleted_at - первичный сурогатнывй ключ

    Commentary:

        {comment_id} -> author_id, post_id, body, created_at, last_updated_at, deleted_at - первичный сурогатный ключ

    Pin_like:

        {pin_like_id} -> liker_id, post_id, created_at - первичный сурогатнывй ключ

    Comment_like:

        {comm_like_id} -> commenter_id, comm_id, created_at - первичный сурогатнывй ключ

    Subscription:

        {subscription_id} -> subscribee_id, subscriber_id, created_at - первичный сурогатнывй ключ

    Pin_desc_relation:

        {desk_relation_id} -> desk_id, pin_id, created_at - первичный сурогатнывй ключ

    Chat:

        {chat_id} -> user_a_id, user_b_id, name, created_at, updated_at - первичный сурогатнывй ключ

    Chat_message:

        {message_id} -> chat_id, author_id, body, created_at, updated_at, related_to - первичный сурогатнывй ключ

    Tag:

        {tag_id} -> name, created_at - первичный сурогатнывй ключ
        {name} -> tag_id, created_at - естественный ключ

    Pin_Tag_relation:

        {tag_relation_id} -> tag_id, pin_id - первичный сурогатнывй ключ

## 2 Нормальная форма:

    все первичные ключи: user_id, desk_id, pin_id, comment_id, pin_like_id, comm_like_id, subscription_id, desk_relation_id, chat_id, message_id, tag_id, tag_relation_id  - атомарны и сурогатные, не массивы => Выполняется вторая нормальная форма 

## 3 Нормальная форма:

    В полях: Desk, Pin, Commentary, Pin_like, Comment_like, Subscription, Pin_desk_relation, Chat, Chat_message, Pin_tag rekation нет транзитивных зависимостей: ни один неключевой атрибут не определяет другой неключевой атрибут. поля зависят только от ключей, а не друг от друга. 

    User:

        Нет транзитивных зависимостей: ни один неключевой атрибут не определяет другой неключевой атрибут. Поля зависят только от ключей, а не друг от друга. А также user_id <-> user_tag

    Tag:

        Нет транзитивных зависимостей: ни один неключевой атрибут не определяет другой неключевой атрибут. поля зависят только от ключей, а не друг от друга. А также tag_id <-> name

## НФБК:

    Поля: Desk, Pin, Commentary, Pin_like, Comment_like, Subscription, Pin_desk_relation, Chat, Chat_message, Pin_tag rekation имеют один потенциальный ключ, находятся в 3 НФ и ключевой атрибут зависит от себя же.

    User: оба потенциальных ключа user_id и user_tag зависят друг от друга: user_id <-> user_tag

    Tag:  оба потенциальных tag_id и name ключа зависят друг от друга: tag_id <-> name
