-- +migrate Up
CREATE TABLE `password_reminders` (
  `email` varchar(191) NOT NULL,
  `token` varchar(191) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  KEY `password_reminders_email_index` (`email`),
  KEY `password_reminders_token_index` (`token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `users` (
  `id` int(10) NOT NULL AUTOINCREMENT,
  `username` varchar(191) NOT NULL,
  `email` varchar(300) DEFAULT NULL,
  `password` varchar(500) NOT NULL,
  `moderator` tinyint(1) NOT NULL,
  `remember_token` varchar(500) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  `updated_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  PRIMARY KEY (`id`),
  UNIQUE KEY `users_username_unique` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `identifiers` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `docset_name` varchar(340) NOT NULL,
  `docset_filename` varchar(340) NOT NULL,
  `docset_platform` varchar(340) NOT NULL,
  `docset_bundle` varchar(340) NOT NULL,
  `docset_version` varchar(340) NOT NULL,
  `page_path` longtext NOT NULL,
  `page_title` varchar(340) NOT NULL,
  `httrack_source` longtext NOT NULL,
  `banned_from_public` tinyint(1) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  `updated_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `entries` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `title` varchar(340) NOT NULL,
  `body` longtext NOT NULL,
  `body_rendered` longtext NOT NULL,
  `type` varchar(255) NOT NULL,
  `identifier_id` int(10) unsigned NOT NULL,
  `anchor` varchar(2000) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `public` tinyint(1) NOT NULL,
  `removed_from_public` tinyint(1) NOT NULL,
  `score` int(11) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  `updated_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  PRIMARY KEY (`id`),
  KEY `entries_identifier_id_foreign` (`identifier_id`),
  KEY `entries_user_id_foreign` (`user_id`),
  CONSTRAINT `entries_identifier_id_foreign` FOREIGN KEY (`identifier_id`) REFERENCES `identifiers` (`id`),
  CONSTRAINT `entries_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `teams` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(191) NOT NULL,
  `access_key` varchar(500) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  `updated_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  PRIMARY KEY (`id`),
  UNIQUE KEY `teams_name_unique` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `entry_team` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `entry_id` int(10) unsigned NOT NULL,
  `team_id` int(10) unsigned NOT NULL,
  `removed_from_team` tinyint(1) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  `updated_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  PRIMARY KEY (`id`),
  KEY `entry_team_entry_id_foreign` (`entry_id`),
  KEY `entry_team_team_id_foreign` (`team_id`),
  CONSTRAINT `entry_team_entry_id_foreign` FOREIGN KEY (`entry_id`) REFERENCES `entries` (`id`),
  CONSTRAINT `entry_team_team_id_foreign` FOREIGN KEY (`team_id`) REFERENCES `teams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `team_user` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `team_id` int(10) unsigned NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `role` varchar(255) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  `updated_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  PRIMARY KEY (`id`),
  KEY `team_user_team_id_foreign` (`team_id`),
  KEY `team_user_user_id_foreign` (`user_id`),
  CONSTRAINT `team_user_team_id_foreign` FOREIGN KEY (`team_id`) REFERENCES `teams` (`id`),
  CONSTRAINT `team_user_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `votes` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `type` tinyint(4) NOT NULL,
  `entry_id` int(10) unsigned NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  `updated_at` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
  PRIMARY KEY (`id`),
  KEY `votes_entry_id_foreign` (`entry_id`),
  KEY `votes_user_id_foreign` (`user_id`),
  CONSTRAINT `votes_entry_id_foreign` FOREIGN KEY (`entry_id`) REFERENCES `entries` (`id`),
  CONSTRAINT `votes_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
-- +migrate Down
DROP TABLE votes;
DROP TABLE team_user;
DROP TABLE entry_team;
DROP TABLE teams;
DROP TABLE identifiers;
DROP TABLE entries;
DROP TABLE users;
