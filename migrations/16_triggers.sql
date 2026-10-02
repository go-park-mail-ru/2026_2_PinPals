CREATE TRIGGER trg_user_updated_at
BEFORE UPDATE ON "user"
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_password_updated_at
BEFORE UPDATE ON "password"
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_desk_updated_at
BEFORE UPDATE ON desk
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_pin_updated_at
BEFORE UPDATE ON pin
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_commentary_updated_at
BEFORE UPDATE ON commentary
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_chat_updated_at
BEFORE UPDATE ON chat
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_chat_message_updated_at
BEFORE UPDATE ON chat_message
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_user_soft_delete
BEFORE UPDATE ON "user"
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();

CREATE TRIGGER trg_desk_soft_delete
BEFORE UPDATE ON desk
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();

CREATE TRIGGER trg_pin_soft_delete
BEFORE UPDATE ON pin
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();

CREATE TRIGGER trg_commentary_soft_delete
BEFORE UPDATE ON commentary
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();

CREATE TRIGGER trg_chat_soft_delete
BEFORE UPDATE ON chat
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();

CREATE TRIGGER trg_chat_message_soft_delete
BEFORE UPDATE ON chat_message
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();
