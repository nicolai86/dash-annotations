CREATE TABLE password_reminders (
  "email" varchar(191) NOT NULL,
  "token" varchar(191) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'
);
