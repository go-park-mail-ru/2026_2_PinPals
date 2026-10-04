CREATE OR REPLACE TRIGGER trg_user_check_age
BEFORE INSERT OR UPDATE OF birth_date ON "user"
FOR EACH ROW
EXECUTE FUNCTION check_user_age();

CREATE OR REPLACE TRIGGER trg_user_updated_at
BEFORE UPDATE ON "user"
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE OR REPLACE TRIGGER trg_password_updated_at
BEFORE UPDATE ON "password"
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE OR REPLACE TRIGGER trg_pin_updated_at
BEFORE UPDATE ON pin
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION fn_set_updated_at();

CREATE OR REPLACE TRIGGER trg_user_soft_delete
BEFORE UPDATE ON "user"
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();

CREATE OR REPLACE TRIGGER trg_pin_soft_delete
BEFORE UPDATE ON pin
FOR EACH ROW
WHEN (NEW.deleted = true AND OLD.deleted = false)
EXECUTE FUNCTION fn_set_deleted_at();
