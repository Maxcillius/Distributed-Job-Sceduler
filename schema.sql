CREATE TABLE jobstatus (
    id BIGSERIAL PRIMARY KEY,
    name text NOT NULL,
    command text NOT NULL,
    Args text[],
    WorkDir text,
    TimeoutSeconds int,
    status text NOT NULL
);