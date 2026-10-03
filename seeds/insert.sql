-- 5 тестовых пользователей
CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO "user"
    (name, user_tag, birth_date, description, avatar_url)
VALUES
    ('Ультра Гигачелов', 'ultra_giga',DATE '2005-03-14', 'Люблю мою крутую девушку', NULL),
    ('Красивый Чел','Chuvaak',DATE '2007-06-22', 'Собираю идеи', NULL),
    ('Чувиха 33','Chuv33',DATE '2001-01-09', 'Природа.', NULL),
    ('Бразер Альфа','PussInCloak', DATE '2004-08-30', 'Люблю что-то с чем-то', NULL),
    ('Иван Петров','ivan_petrov', DATE '1999-11-17', 'Просто Иван Петров', NULL)
ON CONFLICT (user_tag) DO NOTHING;

INSERT INTO user_auth (user_id, password_hash)
VALUES
    (1, crypt('qwerty123',  gen_salt('bf', 10))),
    (2, crypt('password1',  gen_salt('bf', 10))),
    (3, crypt('testpass1',  gen_salt('bf', 10))),
    (4, crypt('qazwsx123',  gen_salt('bf', 10))),
    (5, crypt('hello1234',  gen_salt('bf', 10)));
ON CONFLICT (user_id) DO NOTHING;


INSERT INTO pin (creator_id, image_url, name, description)
VALUES
    (1, 'https://example.com/images/pin01.jpg', 'Стоковое фото 1', 'Абсолютная имба'),
    (1, 'https://example.com/images/pin02.jpg', 'Стоковое фото 2', 'невероятная кайфуха'),
    (1, 'https://example.com/images/pin03.jpg', 'Стоковое фото 3', 'Лютейшая невероятность'),
    (1, 'https://example.com/images/pin04.jpg', 'Стоковое фото 4', 'Кринжатина'),
    (1, 'https://example.com/images/pin05.jpg', 'Стоковое фото 5', 'Чел, ну да'),

    (2, 'https://example.com/images/pin06.jpg', 'Стоковое фото 6', 'Ну см кем не бывает ^:P'),
    (2, 'https://example.com/images/pin07.jpg', 'Стоковое фото 7', 'модерн...'),
    (2, 'https://example.com/images/pin08.jpg', 'Стоковое фото 8', 'Неее, это лишнее'),
    (2, 'https://example.com/images/pin09.jpg', 'Стоковое фото 9', 'Просто отврат'),
    (2, 'https://example.com/images/pin10.jpg', 'Стоковое фото 10', 'моё любимое фото меня и моей кошечки'),

    (3, 'https://example.com/images/pin11.jpg', 'Стоковое фото 11', 'Чья-то бабуля?'),
    (3, 'https://example.com/images/pin12.jpg', 'Стоковое фото 12', 'Ужас кринжвиля'),
    (3, 'https://example.com/images/pin13.jpg', 'Стоковое фото 13', 'Жёсткий замес'),
    (3, 'https://example.com/images/pin14.jpg', 'Стоковое фото 14', 'Человеческое фото'),
    (3, 'https://example.com/images/pin15.jpg', 'Стоковое фото 15', 'Кончаются идеи'),

    (4, 'https://example.com/images/pin16.jpg', 'Стоковое фото 16', 'Разнос'),
    (4, 'https://example.com/images/pin17.jpg', 'Стоковое фото 17', 'Стоило подумать об этом...'),
    (4, 'https://example.com/images/pin18.jpg', 'Стоковое фото 18', 'Да сколько можно-то?!'),
    (4, 'https://example.com/images/pin19.jpg', 'Стоковое фото 19', 'Пишу это ручками, а лучше бы делом занялся'),
    (4, 'https://example.com/images/pin20.jpg', 'Стоковое фото 20', 'Крутейшя крутость яйцо в крутую без регистрации и смс'),

    (5, 'https://example.com/images/pin21.jpg', 'Стоковое фото 21', 'Мой любимый сериал'),
    (5, 'https://example.com/images/pin22.jpg', 'Стоковое фото 22', 'Сальса в пятнциу вечером'),
    (5, 'https://example.com/images/pin23.jpg', 'Стоковое фото 23', 'Дорогая, у нас тройня'),
    (5, 'https://example.com/images/pin24.jpg', 'Стоковое фото 24', 'как много стоковых фоток'),
    (5, 'https://example.com/images/pin25.jpg', 'Стоковое фото 25', 'Ужасно мило'),

    (1, 'https://example.com/images/pin26.jpg', 'Стоковое фото 26', 'Мой любимый праздник'),
    (2, 'https://example.com/images/pin27.jpg', 'Стоковое фото 27', 'Это ты мне?!'),
    (3, 'https://example.com/images/pin28.jpg', 'Стоковое фото 28', 'Предпредпоследнее описание'),
    (4, 'https://example.com/images/pin29.jpg', 'Стоковое фото 29', 'Ну нет, ну пожалуйста, нееет'),
    (5, 'https://example.com/images/pin30.jpg', 'Стоковое фото 30', 'О да, наконец');
ON CONFLICT (image_url) DO NOTHING;
