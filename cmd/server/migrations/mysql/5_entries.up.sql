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

