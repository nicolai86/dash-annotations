
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
