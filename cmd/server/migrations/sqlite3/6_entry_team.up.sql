CREATE TABLE entry_team (
  "id" INTEGER primary key,
  "entry_id" int(10)  NOT NULL,
  "team_id" int(10)  NOT NULL,
  "removed_from_team" tinyint(1) NOT NULL DEFAULT false,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "entry_team_entry_id_foreign" FOREIGN KEY ("entry_id") REFERENCES "entries" ("id"),
  CONSTRAINT "entry_team_team_id_foreign" FOREIGN KEY ("team_id") REFERENCES "teams" ("id")
);


