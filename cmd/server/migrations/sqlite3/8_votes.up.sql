CREATE TABLE votes (
  "id" INTEGER primary key,
  "type" tinyint(4) NOT NULL,
  "entry_id" int(10)  NOT NULL,
  "user_id" int(10)  NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "votes_entry_id_foreign" FOREIGN KEY ("entry_id") REFERENCES "entries" ("id"),
  CONSTRAINT "votes_user_id_foreign" FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

