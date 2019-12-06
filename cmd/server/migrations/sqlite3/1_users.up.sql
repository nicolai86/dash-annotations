CREATE TABLE users (
  "id" INTEGER primary key ,
  "username" varchar(191) NOT NULL,
  "email" varchar(300) DEFAULT NULL,
  "password" varchar(500) NOT NULL,
  "moderator" tinyint(1) NOT NULL DEFAULT false,
  "remember_token" varchar(500) DEFAULT NULL,
  "created_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  "updated_at" timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'
);

