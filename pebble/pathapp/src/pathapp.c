#include "pebble.h"

#define MAXBUFFERSIZE 200
#define MAX_SCHED_ITEMS 10
#define MAX_STN_ITEMS 20
#define MAX_DIR_ITEMS 5

#define STN_NAME_LEN 10

static Window *main_window, *direction_window, *schedule_window;
static SimpleMenuLayer *stn_layer, *dir_layer, *sched_layer;

static SimpleMenuItem stn_items[MAX_STN_ITEMS], dir_items[MAX_DIR_ITEMS], sched_items[MAX_SCHED_ITEMS];
static SimpleMenuSection stn_sections[1], dir_sections[1], sched_sections[1];

static char buffer[MAXBUFFERSIZE];

static char from_station[STN_NAME_LEN];
static char *delim = ";";

enum {
	PATH_STN_KEY = 0x0,
};

static size_t sched_num_items;

static char *stn_names[] = {
	"JSQ",
	"Grove St",
	"Newport",
	"Hoboken",
	"Chris St",
	"9 St",
	"14 St",
	"23 St",
	"33 St",
	NULL,
};

struct direction {
	char *title;
	char *stub;
};

static struct direction directions[] = {
	{ .title = "To 33rd St", .stub = "jsq_33rd" },
	{ .title = "To JSQ", .stub = "33rd_jsq" },
};

static void send_to_phone(int key, const char *direction) {
	DictionaryIterator *iter;
	char *buf;

	app_message_outbox_begin(&iter);
	if (!iter) {
		return;
	}

	buf = buffer;
	memcpy(buf, from_station, strlen(from_station));
	buf += strlen(from_station);
	memcpy(buf, delim, strlen(delim));
	buf += strlen(delim);
	strncpy(buf, direction, strlen(direction));

	buf = buffer;
	Tuplet tuplet = TupletCString(key, buf);
	dict_write_tuplet(iter, &tuplet);

	dict_write_end(iter);

	app_message_outbox_send();

	// Zero out the buffer area we used. We can leave the last
	// NUL byte as it is, so we don't need to do strlen(buffer)+1 here.
	memset(buffer, 0x00, strlen(buffer));
}

static void sched_menu_callback(int index, void *ctx) {
	vibes_short_pulse();
	sched_items[index].subtitle = "Ok. Now hurry!";
	layer_mark_dirty(simple_menu_layer_get_layer(sched_layer));
}

/*
static void stn_callback(int index, void *ctx) {
	send_to_phone(PATH_STN_KEY, stn_names[index]);
}
*/

static void dir_callback(int index, void *ctx) {
	send_to_phone(PATH_STN_KEY, directions[index].stub);
}

static void direction_window_load(Window *window) {
	Layer *window_layer;
	GRect bounds;
	size_t n;

	for (n = 0; n < sizeof(directions)/sizeof(directions[n]); n++) {
		dir_items[n] = (SimpleMenuItem) {
			.title = directions[n].title,
			.callback = dir_callback,
		};
	}

	dir_sections[0] = (SimpleMenuSection) {
		.title = "Choose Direction",
		.num_items = n,
		.items = dir_items,
	};

	window_layer = window_get_root_layer(window);
	bounds = layer_get_frame(window_layer);

	dir_layer = simple_menu_layer_create(bounds, window, dir_sections, 1, NULL);

	layer_add_child(window_layer, simple_menu_layer_get_layer(dir_layer));
}

static void direction_window_unload(Window *window) {
	memset(from_station, 0x00, sizeof from_station);
	simple_menu_layer_destroy(dir_layer);
}

static void stn_callback(int index, void *ctx) {
	strncpy(from_station, stn_names[index], strlen(stn_names[index]));
	window_stack_push(direction_window, true);
}

static void populate_menu() {
	size_t n, len;
	char *t, *dest;

	len = strlen(buffer);
	sched_num_items = 0;
	n = 0;

	while (n < len) {
		t = (char *)&buffer[n];

		while (buffer[n++] != ',');
		buffer[n - 1] = '\0';

		dest = (char *)&buffer[n];

		while (buffer[n++] != ';');
		buffer[n - 1] = '\0';

		sched_items[sched_num_items++] = (SimpleMenuItem) {
			.title = t,
			.subtitle = dest,
			.callback = sched_menu_callback,
		};
	}

	sched_sections[0] = (SimpleMenuSection) {
		.num_items = sched_num_items,
		.items = sched_items,
	};
}

static void main_window_load(Window *window) {
	Layer *window_layer;
	GRect bounds;
	char *item;
	size_t n;

	n = 0;
	while (1) {
		item = stn_names[n];
		if (!item)
			break;

		stn_items[n++] = (SimpleMenuItem) {
			.title = item,
			.callback = stn_callback,
		};
	}

	stn_sections[0] = (SimpleMenuSection) {
		.title = "From Station",
		.num_items = n,
		.items = stn_items,
	};


	window_layer = window_get_root_layer(window);
	bounds = layer_get_frame(window_layer);

	stn_layer = simple_menu_layer_create(bounds, window, stn_sections, 1, NULL);

	layer_add_child(window_layer, simple_menu_layer_get_layer(stn_layer));
}

static void main_window_unload(Window *window) {
	simple_menu_layer_destroy(stn_layer);
}

static void schedule_window_load(Window *window) {
	Layer *window_layer;
	GRect bounds;

	populate_menu();

	window_layer = window_get_root_layer(window);
	bounds = layer_get_frame(window_layer);

	sched_layer = simple_menu_layer_create(bounds, window, sched_sections, 1, NULL);

	layer_add_child(window_layer, simple_menu_layer_get_layer(sched_layer));
}

static void schedule_window_unload(Window *window) {
	memset(buffer, 0x00, sizeof buffer);
	simple_menu_layer_destroy(sched_layer);
}

static void in_received_handler(DictionaryIterator *iter, void *context) {
	Tuple *sched_tuple;

	sched_tuple = dict_find(iter, PATH_STN_KEY);
	if (sched_tuple) {
		memcpy(buffer, sched_tuple->value->data, sched_tuple->length);
		window_stack_push(schedule_window, true);
	}
}

static void in_dropped_handler(AppMessageResult reason, void *context) {
	APP_LOG(APP_LOG_LEVEL_WARNING, "Message dropped [%d]", reason);
}

static void out_failed_handler(
	DictionaryIterator *failed, AppMessageResult reason, void *context)
{
	APP_LOG(APP_LOG_LEVEL_WARNING, "Message failed to send [%d]", reason);
}

static void init() {
	uint32_t inbound_buffer, outbound_buffer;

	inbound_buffer = outbound_buffer = MAXBUFFERSIZE;
	app_message_open(inbound_buffer, outbound_buffer);

	app_message_register_inbox_received(in_received_handler);
	app_message_register_inbox_dropped(in_dropped_handler);
	app_message_register_outbox_failed(out_failed_handler);

	main_window = window_create();

	window_set_window_handlers(main_window, (WindowHandlers) {
		.load = main_window_load,
		.unload = main_window_unload,
	});

	direction_window = window_create();

	window_set_window_handlers(direction_window, (WindowHandlers) {
		.load = direction_window_load,
		.unload = direction_window_unload,
	});

	schedule_window = window_create();

	window_set_window_handlers(schedule_window, (WindowHandlers) {
		.load = schedule_window_load,
		.unload = schedule_window_unload,
	});

	window_stack_push(main_window, true);
}

static void deinit() {
	window_destroy(schedule_window);
	window_destroy(direction_window);
	window_destroy(main_window);
	app_message_deregister_callbacks();
}

int main(void) {
	init();
	app_event_loop();
	deinit();
}

