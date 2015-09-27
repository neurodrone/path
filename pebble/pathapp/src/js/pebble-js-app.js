Pebble.addEventListener("ready", function(e) {
	console.log("connected! " + e.ready);
	console.log(e.type);
});

Pebble.addEventListener("appmessage", function(e) {
	if (e.payload.sched === undefined) {
		console.log("Cannot understand payload: '" + e.payload + "'");
		return;
	}
	var d = new Date();
	var req = new XMLHttpRequest();
	var msg = {};
	var ampm = "AM";
	var delim = ";";
	var station_direction;

	console.log("Payload: '" + e.payload.sched + "'");

	station_direction = e.payload.sched.split(delim);

	var mins = d.getMinutes();
	if (mins < 10) {
		mins = "0" + mins;
	}
	var hours = d.getHours();
	if (hours > 12) {
		ampm = "PM";
		hours -= 12;
	}
	var time_str = hours+':'+mins+ampm;
	var url = 'http://localhost:8080/p/'
		+encodeURIComponent(station_direction[0])+'/'
		+encodeURIComponent(station_direction[1])+'/'
		+encodeURIComponent(time_str)+'/';

	req.open('GET', url, true);
	req.onload = function(e) {
		if (req.readyState == 4) {
			if (req.status != 200) {
				console.log(req.responseText);
				return;
			}
			console.log("success!");
			console.log(req.responseText);

			msg.sched = req.responseText;
			Pebble.sendAppMessage(msg);
			console.log("Sent data to phone!");
		}
	};
	req.send(null);
});
