CREATE TABLE t (
  id BIGSERIAL PRIMARY KEY,
  a varchar(200) NOT NULL DEFAULT 'x, y',
  b INTEGER,
  dropped INTEGER,
  old_name INTEGER,
  create_time TIMESTAMP,
  UNIQUE (a)
);
CREATE TABLE gone (id INTEGER);
