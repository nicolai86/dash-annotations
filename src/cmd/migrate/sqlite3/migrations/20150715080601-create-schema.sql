-- +migrate Up
CREATE TABLE "entries" (
  "id" INTEGER primary key,
  "title" varchar(340) NOT NULL,
  "body" longtext NOT NULL,
  "body_rendered" longtext NOT NULL,
  "type" varchar(255) NOT NULL,
  "identifier_id" int(10)  NOT NULL,
  "anchor" varchar(2000) NOT NULL,
  "user_id" int(10)  NOT NULL,
  "public" tinyint(1) NOT NULL default false,
  "removed_from_public" tinyint(1) NOT NULL default false,
  "score" int(11) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "entries_identifier_id_foreign" FOREIGN KEY ("identifier_id") REFERENCES "identifiers" ("id"),
  CONSTRAINT "entries_user_id_foreign" FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

CREATE TABLE "entry_team" (
  "id" INTEGER primary key,
  "entry_id" int(10)  NOT NULL,
  "team_id" int(10)  NOT NULL,
  "removed_from_team" tinyint(1) NOT NULL default false,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "entry_team_entry_id_foreign" FOREIGN KEY ("entry_id") REFERENCES "entries" ("id"),
  CONSTRAINT "entry_team_team_id_foreign" FOREIGN KEY ("team_id") REFERENCES "teams" ("id")
);

CREATE TABLE "identifiers" (
  "id" INTEGER primary key,
  "docset_name" varchar(340) NOT NULL,
  "docset_filename" varchar(340) NOT NULL,
  "docset_platform" varchar(340) NOT NULL,
  "docset_bundle" varchar(340) NOT NULL,
  "docset_version" varchar(340) NOT NULL,
  "page_path" longtext NOT NULL,
  "page_title" varchar(340) NOT NULL,
  "httrack_source" longtext NOT NULL,
  "banned_from_public" tinyint(1) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'
);

CREATE TABLE "password_reminders" (
  "email" varchar(191) NOT NULL,
  "token" varchar(191) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'
);

CREATE TABLE "team_user" (
  "id" INTEGER primary key,
  "team_id" int(10)  NOT NULL,
  "user_id" int(10)  NOT NULL,
  "role" varchar(255) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "team_user_team_id_foreign" FOREIGN KEY ("team_id") REFERENCES "teams" ("id"),
  CONSTRAINT "team_user_user_id_foreign" FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

CREATE TABLE "teams" (
  "id" INTEGER primary key,
  "name" varchar(191) NOT NULL,
  "access_key" varchar(500) NOT NULL DEFAULT '',
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'
);

CREATE TABLE "users" (
  "id" INTEGER primary key ,
  "username" varchar(191) NOT NULL,
  "email" varchar(300) DEFAULT NULL,
  "password" varchar(500) NOT NULL,
  "moderator" tinyint(1) NOT NULL DEFAULT false,
  "remember_token" varchar(500) DEFAULT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'
);

CREATE TABLE "votes" (
  "id" INTEGER primary key,
  "type" tinyint(4) NOT NULL,
  "entry_id" int(10)  NOT NULL,
  "user_id" int(10)  NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  CONSTRAINT "votes_entry_id_foreign" FOREIGN KEY ("entry_id") REFERENCES "entries" ("id"),
  CONSTRAINT "votes_user_id_foreign" FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

CREATE INDEX "votes_votes_entry_id_foreign" ON "votes" ("entry_id");
CREATE INDEX "votes_votes_user_id_foreign" ON "votes" ("user_id");
CREATE INDEX "password_reminders_password_reminders_email_index" ON "password_reminders" ("email");
CREATE INDEX "password_reminders_password_reminders_token_index" ON "password_reminders" ("token");
CREATE INDEX "users_users_username_unique" ON "users" ("username");
CREATE INDEX "entries_entries_identifier_id_foreign" ON "entries" ("identifier_id");
CREATE INDEX "entries_entries_user_id_foreign" ON "entries" ("user_id");
CREATE INDEX "teams_teams_name_unique" ON "teams" ("name");
CREATE INDEX "entry_team_entry_team_entry_id_foreign" ON "entry_team" ("entry_id");
CREATE INDEX "entry_team_entry_team_team_id_foreign" ON "entry_team" ("team_id");
CREATE INDEX "team_user_team_user_team_id_foreign" ON "team_user" ("team_id");
CREATE INDEX "team_user_team_user_user_id_foreign" ON "team_user" ("user_id");
-- +migrate Down

DROP TABLE votes;
DROP TABLE team_user;
DROP TABLE entry_team;
DROP TABLE teams;
DROP TABLE identifiers;
DROP TABLE entries;
DROP TABLE users;
