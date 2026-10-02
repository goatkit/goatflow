package main

// tableMode is what the importer does with one OTRS table.
type tableMode int

const (
	// modeMerge: configuration rows a fresh GoatFlow may already hold (seeded
	// lookups, admin-created entries). An OTRS row replaces the target row
	// with its key, or is inserted; a target row with another key holding the
	// same unique value is renamed first.
	modeMerge tableMode = iota
	// modeReplace: link and preference rows without an identity of their own
	// (permissions, preferences, queue/template links). The OTRS rows are
	// authoritative: the target's rows are deleted and the OTRS rows inserted,
	// so an imported agent gets exactly the OTRS permissions.
	modeReplace
	// modeData: rows only the OTRS system has (tickets, articles, customers,
	// ...). The target table must be empty; --force empties it first.
	modeData
	// modeSysconfig: sysconfig_modified, matched to the target's
	// sysconfig_default by setting name (see importSysconfig).
	modeSysconfig
	// modeSkip: not imported, for the reason given.
	modeSkip
	// modeGoatFlow: a GoatFlow table OTRS does not have.
	modeGoatFlow
)

func (m tableMode) String() string {
	switch m {
	case modeMerge:
		return "merge"
	case modeReplace:
		return "replace"
	case modeData:
		return "import"
	case modeSysconfig:
		return "sysconfig"
	case modeSkip:
		return "skip"
	default:
		return "goatflow"
	}
}

// tablePlan is the importer's decision for one table of the OTRS 6 /
// Znuny 6.x schema (the GoatFlow schema minus its gk_* tables).
type tablePlan struct {
	name   string
	mode   tableMode
	key    string // modeMerge: key column
	unique string // modeMerge: UNIQUE column an OTRS row may take over from another target row
	reason string // modeSkip / modeGoatFlow: why the table is not imported
}

func merge(name, unique string) tablePlan {
	return tablePlan{name: name, mode: modeMerge, key: "id", unique: unique}
}

func replace(name string) tablePlan { return tablePlan{name: name, mode: modeReplace} }

func data(name string) tablePlan { return tablePlan{name: name, mode: modeData} }

func skip(name, reason string) tablePlan {
	return tablePlan{name: name, mode: modeSkip, reason: reason}
}

func goatflowOnly(name string) tablePlan {
	return tablePlan{name: name, mode: modeGoatFlow, reason: "GoatFlow table, not part of the OTRS schema"}
}

// importPlan covers every table of the GoatFlow schema except gk_*. The
// import order is derived from the target's foreign keys (see importOrder);
// this list only breaks ties.
var importPlan = []tablePlan{
	// Agents, groups, roles and the lookups everything else references.
	merge("users", "login"),
	merge("valid", "name"),
	merge("groups", "name"),
	merge("roles", "name"),
	merge("permission_groups", "name"),
	merge("ticket_state_type", "name"),
	merge("ticket_state", "name"),
	merge("ticket_priority", "name"),
	merge("ticket_type", "name"),
	merge("ticket_lock_type", "name"),
	merge("ticket_history_type", "name"),
	merge("article_sender_type", "name"),
	merge("communication_channel", "name"),
	merge("article_color", "name"),
	merge("follow_up_possible", "name"),
	merge("salutation", "name"),
	merge("signature", "name"),
	merge("system_address", ""),
	merge("service", "name"),
	merge("sla", "name"),
	merge("queue", "name"),
	merge("auto_response_type", "name"),
	merge("auto_response", "name"),
	merge("standard_template", "name"),
	merge("standard_attachment", "name"),
	merge("notification_event", "name"),
	merge("link_type", "name"),
	merge("link_state", "name"),
	merge("link_object", "name"),
	merge("dynamic_field", "name"),
	merge("acl", "name"),
	merge("calendar", "name"),
	merge("mail_account", ""),
	merge("gi_webservice_config", "name"),
	merge("pm_process", "entity_id"),
	merge("pm_activity", "entity_id"),
	merge("pm_activity_dialog", "entity_id"),
	merge("pm_transition", "entity_id"),
	merge("pm_transition_action", "entity_id"),
	merge("system_maintenance", ""),
	merge("oauth2_token_config", "name"),
	merge("translation", ""),

	// Permissions, preferences and configuration links.
	replace("group_user"),
	replace("group_role"),
	replace("role_user"),
	replace("group_customer_user"),
	replace("group_customer"),
	replace("user_preferences"),
	replace("personal_queues"),
	replace("personal_services"),
	replace("queue_preferences"),
	replace("service_preferences"),
	replace("sla_preferences"),
	replace("service_sla"),
	replace("service_customer_user"),
	replace("queue_standard_template"),
	replace("standard_template_attachment"),
	replace("queue_auto_response"),
	replace("notification_event_item"),
	replace("notification_event_message"),
	replace("pm_process_preferences"),

	// OTRS-only data.
	data("customer_company"),
	data("customer_user"),
	data("customer_preferences"),
	data("customer_user_customer"),
	data("ticket"),
	data("article"),
	data("article_data_mime"),
	data("article_data_mime_plain"),
	data("article_data_mime_attachment"),
	data("article_data_mime_send_error"),
	data("article_data_otrs_chat"),
	data("article_flag"),
	data("ticket_history"),
	data("ticket_flag"),
	data("ticket_watcher"),
	data("time_accounting"),
	data("dynamic_field_obj_id_name"),
	data("dynamic_field_value"),
	data("link_relation"),
	data("mention"),
	data("calendar_appointment"),
	data("calendar_appointment_ticket"),
	data("calendar_appointment_plugin"),
	data("generic_agent_jobs"),
	data("postmaster_filter"),
	data("search_profile"),
	data("xml_storage"),
	data("virtual_fs"),
	data("virtual_fs_preferences"),
	data("virtual_fs_db"),
	data("smime_keys"),
	data("smime_signer_cert_relations"),
	data("oauth2_token"),
	data("gi_webservice_config_history"),
	data("acl_ticket_attribute_relations"),
	data("activity"),

	{name: "sysconfig_modified", mode: modeSysconfig},
	skip("sysconfig_default", "OTRS's XML setting definitions, not data; only the definitions of modified settings GoatFlow does not define are copied, as parents of their sysconfig_modified rows"),
	skip("sysconfig_default_version", "OTRS deployment history"),
	skip("sysconfig_modified_version", "OTRS deployment history"),
	skip("sysconfig_deployment", "OTRS deployment history (generated Perl config files)"),
	skip("sysconfig_deployment_lock", "OTRS deployment lock"),

	skip("sessions", "login sessions; agents and customers log in again"),
	skip("web_upload_cache", "temporary uploads of unsaved forms"),
	skip("form_draft", "unsent OTRS form drafts, Perl-serialized for OTRS screens; GoatFlow has no reader"),
	skip("article_search_index", "GoatFlow does not read it: search runs on article_data_mime (database backend) or Elasticsearch/Zinc"),
	skip("ticket_index", "OTRS StaticDB queue-view cache; GoatFlow does not read it"),
	skip("ticket_lock_index", "OTRS StaticDB lock cache; GoatFlow does not read it"),
	skip("ticket_loop_protection", "per-day auto-reply counters"),
	skip("ticket_number_counter", "OTRS writes one row per ticket number under a random counter_uid; the import sets GoatFlow's counters (one per SystemID, or per SystemID and day) from the imported ticket numbers instead"),
	skip("mail_queue", "outgoing mail not yet sent by the OTRS daemon; flush it before exporting"),
	skip("communication_log", "mail transport log OTRS purges after a few days"),
	skip("communication_log_object", "mail transport log OTRS purges after a few days"),
	skip("communication_log_object_entry", "mail transport log OTRS purges after a few days"),
	skip("communication_log_obj_lookup", "mail transport log OTRS purges after a few days"),
	skip("gi_debugger_entry", "web service debug log"),
	skip("gi_debugger_entry_content", "web service debug log"),
	skip("scheduler_task", "OTRS daemon task queue (Perl-serialized)"),
	skip("scheduler_future_task", "OTRS daemon task queue (Perl-serialized)"),
	skip("scheduler_recurrent_task", "OTRS daemon cron state"),
	skip("process_id", "OTRS daemon process ids"),
	skip("acl_sync", "OTRS ACL deployment state"),
	skip("pm_entity_sync", "OTRS process deployment state"),
	skip("package_repository", "OTRS Perl add-on packages (.opm), not installable in GoatFlow"),
	skip("cloud_service_config", "OTRS Group cloud services"),
	skip("system_data", "OTRS registration and system state"),

	goatflowOnly("admin_action_log"),
	goatflowOnly("admin_action_type"),
	goatflowOnly("canned_response"),
	goatflowOnly("canned_response_category"),
	goatflowOnly("user_api_tokens"),
	goatflowOnly("sysconfig_org"),
	goatflowOnly("dynamic_field_screen_config"),
}

func planFor(name string) (tablePlan, bool) {
	for _, p := range importPlan {
		if p.name == name {
			return p, true
		}
	}
	return tablePlan{}, false
}

func (p tablePlan) imported() bool {
	return p.mode == modeMerge || p.mode == modeReplace || p.mode == modeData || p.mode == modeSysconfig
}
