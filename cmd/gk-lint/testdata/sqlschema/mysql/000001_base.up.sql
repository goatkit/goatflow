-- t exists on both drivers; mysql_only only here.
CREATE TABLE IF NOT EXISTS `t` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `a` VARCHAR(200) NOT NULL DEFAULT 'x, y',
  `b` INT,
  `mysql_only` INT,
  `dropped` INT,
  `old_name` INT,
  create_time DATETIME,
  PRIMARY KEY (`id`),
  UNIQUE KEY uk_t_a (`a`),
  CONSTRAINT fk_t FOREIGN KEY (b) REFERENCES t (id)
) ENGINE=InnoDB;
CREATE TABLE gone (id INT);
