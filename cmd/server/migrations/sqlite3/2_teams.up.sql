CREATE TABLE teams (
  "id" INTEGER primary key,
  "name" varchar(191) NOT NULL,
  "access_key" varchar(500) NOT NULL DEFAULT '',
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'
);

