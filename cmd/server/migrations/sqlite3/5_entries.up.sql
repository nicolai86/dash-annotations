CREATE TABLE entries (
  "id" INTEGER primary key,
  "title" varchar(340) NOT NULL,
  "body" longtext NOT NULL,
  "body_rendered" longtext NOT NULL,
  "type" varchar(255) NOT NULL,
  "identifier_id" int(10)  NOT NULL,
  "anchor" varchar(2000) NOT NULL,
  "user_id" int(10)  NOT NULL,
  "public" tinyint(1) NOT NULL DEFAULT false,
  "removed_from_public" tinyint(1) NOT NULL DEFAULT false,
  "score" int(11) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "entries_identifier_id_foreign" FOREIGN KEY ("identifier_id") REFERENCES "identifiers" ("id"),
  CONSTRAINT "entries_user_id_foreign" FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

