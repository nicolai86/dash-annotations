CREATE TABLE team_user (
  "id" INTEGER primary key,
  "team_id" int(10)  NOT NULL,
  "user_id" int(10)  NOT NULL,
  "role" varchar(255) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "team_user_team_id_foreign" FOREIGN KEY ("team_id") REFERENCES "teams" ("id"),
  CONSTRAINT "team_user_user_id_foreign" FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

