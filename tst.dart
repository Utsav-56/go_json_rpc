import 'dart:convert';
import 'dart:io';

void main() async {
  var socket = await Socket.connect('localhost', 9000);

  socket.listen((data) {
    var msg = jsonDecode(utf8.decode(data));
    print(msg);
  });
}
