CREATE OR REPLACE FUNCTION fn_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

COMMENT ON FUNCTION fn_set_updated_at() IS 'проставляет updated_at = now()';

CREATE OR REPLACE FUNCTION fn_set_deleted_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.deleted = true AND OLD.deleted = false THEN
        NEW.deleted_at := now();
    END IF;
    RETURN NEW;
END;
$$;

COMMENT ON FUNCTION fn_set_deleted_at() IS 'при deleted false -> true проставляет deleted_at = now()';
