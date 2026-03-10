import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'handlers.dart';
import 'types.dart';

/// JSON-RPC Client for process management
class RpcClient {
  final String host;
  final int port;

  Socket? _socket;
  StreamSubscription<List<int>>? _subscription;
  bool _connected = false;

  final Map<String, Completer<Response>> _pendingRequests = {};
  final ResponseHandlerRegistry _responseHandlers = ResponseHandlerRegistry();
  final NotificationListenerRegistry _notificationHandlers =
      NotificationListenerRegistry();

  int _requestIdCounter = 0;
  final StringBuffer _buffer = StringBuffer();

  RpcClient({
    required this.host,
    required this.port,
  });

  /// Get response handler registry for registering handlers
  ResponseHandlerRegistry get responseHandlers => _responseHandlers;

  /// Get notification listener registry
  NotificationListenerRegistry get notificationHandlers => _notificationHandlers;

  /// Check if client is connected
  bool get isConnected => _connected;

  /// Connect to the RPC server
  Future<void> connect() async {
    if (_connected) {
      throw StateError('Already connected');
    }

    try {
      _socket = await Socket.connect(host, port);
      _connected = true;

      // Listen for incoming data
      _subscription = _socket!.listen(
        _handleData,
        onError: _handleError,
        onDone: _handleDone,
        cancelOnError: false,
      );
    } catch (e) {
      throw Exception('Failed to connect to $host:$port - $e');
    }
  }

  /// Disconnect from the RPC server
  Future<void> disconnect() async {
    if (!_connected) {
      return;
    }

    _connected = false;

    await _subscription?.cancel();
    _subscription = null;

    await _socket?.close();
    _socket = null;

    // Complete all pending requests with error
    for (final completer in _pendingRequests.values) {
      if (!completer.isCompleted) {
        completer.completeError(
          Exception('Connection closed'),
        );
      }
    }
    _pendingRequests.clear();
    _buffer.clear();
  }

  /// Generate a unique request ID
  String _generateRequestId() {
    return 'req-${++_requestIdCounter}';
  }

  /// Send a message to the server
  Future<void> _sendMessage(Message message) async {
    if (!_connected) {
      throw StateError('Not connected');
    }

    final json = jsonEncode(message.toJson());
    _socket!.write('$json\n');
    await _socket!.flush();
  }

  /// Send a request and wait for response
  Future<Response> _sendRequest(Message message) async {
    final completer = Completer<Response>();
    _pendingRequests[message.id!] = completer;

    try {
      await _sendMessage(message);
    } catch (e) {
      _pendingRequests.remove(message.id);
      completer.completeError(e);
    }

    return completer.future;
  }

  /// Handle incoming data from socket
  void _handleData(List<int> data) {
    final text = utf8.decode(data);
    _buffer.write(text);

    // Process complete JSON messages (newline-delimited)
    final lines = _buffer.toString().split('\n');

    // Keep the last incomplete line in buffer
    _buffer.clear();
    if (lines.isNotEmpty && !text.endsWith('\n')) {
      _buffer.write(lines.last);
      lines.removeLast();
    }

    // Process complete messages
    for (final line in lines) {
      if (line.trim().isEmpty) continue;

      try {
        final json = jsonDecode(line) as Map<String, dynamic>;
        _processMessage(json);
      } catch (e) {
        print('Error decoding message: $e');
      }
    }
  }

  /// Process a decoded JSON message
  void _processMessage(Map<String, dynamic> json) {
    final type = json['type'] as String?;

    if (type == MessageType.notification) {
      // Handle notification
      _handleNotification(json);
    } else if (json.containsKey('id')) {
      // Handle response
      _handleResponse(json);
    }
  }

  /// Handle a response message
  void _handleResponse(Map<String, dynamic> json) {
    final response = Response.fromJson(json);

    // Complete pending request if exists
    final completer = _pendingRequests.remove(response.id);
    if (completer != null && !completer.isCompleted) {
      completer.complete(response);
    }

    // Dispatch to registered handlers
    _responseHandlers.dispatch(response);
  }

  /// Handle a notification message
  void _handleNotification(Map<String, dynamic> json) {
    try {
      final notification = NotificationMessage.fromJson(json);
      _notificationHandlers.dispatch(notification);
    } catch (e) {
      print('Error handling notification: $e');
    }
  }

  /// Handle socket error
  void _handleError(dynamic error) {
    print('Socket error: $error');
  }

  /// Handle socket done (connection closed)
  void _handleDone() {
    print('Connection closed by server');
    _connected = false;
    disconnect();
  }

  // ==================== Command Methods ====================

  /// Start a process
  Future<Response> startProcess(ProcessRequest request) {
    final message = Message(
      id: _generateRequestId(),
      type: MessageType.command,
      params: CommandParams(
        action: CommandAction.start,
        process: request,
      ).toJson(),
    );

    return _sendRequest(message);
  }

  /// Stop a process
  Future<Response> stopProcess(String processName) {
    final message = Message(
      id: _generateRequestId(),
      type: MessageType.command,
      params: CommandParams(
        action: CommandAction.stop,
        name: processName,
      ).toJson(),
    );

    return _sendRequest(message);
  }

  /// Get status of a specific process
  Future<Response> getProcessStatus(String processName) {
    final message = Message(
      id: _generateRequestId(),
      type: MessageType.command,
      params: CommandParams(
        action: CommandAction.getStatus,
        name: processName,
      ).toJson(),
    );

    return _sendRequest(message);
  }

  /// Get status of all processes
  Future<Response> getAllStatuses() {
    final message = Message(
      id: _generateRequestId(),
      type: MessageType.command,
      params: CommandParams(
        action: CommandAction.getStatus,
      ).toJson(),
    );

    return _sendRequest(message);
  }

  /// Get logs of a process
  Future<Response> getProcessLogs(String processName) {
    final message = Message(
      id: _generateRequestId(),
      type: MessageType.command,
      params: CommandParams(
        action: CommandAction.getLogs,
        name: processName,
      ).toJson(),
    );

    return _sendRequest(message);
  }

  // ==================== Event Methods ====================

  /// Send a custom event
  Future<Response> sendEvent(String eventName, {dynamic data}) {
    final message = Message(
      id: _generateRequestId(),
      type: MessageType.event,
      params: EventParams(
        name: eventName,
        data: data,
      ).toJson(),
    );

    return _sendRequest(message);
  }

  /// Send shutdown event
  Future<Response> shutdown({dynamic data}) {
    return sendEvent('shutdown', data: data);
  }

  /// Send download file event
  Future<Response> downloadFile(String url, String destination) {
    return sendEvent('download_file', data: {
      'url': url,
      'destination': destination,
    });
  }

  /// Send health check event
  Future<Response> healthCheck() {
    return sendEvent('health_check');
  }

  // ==================== Convenience Methods ====================

  /// Register a log listener
  void onLog(LogHandler handler) {
    _notificationHandlers.onLog(handler);
  }

  /// Register a status change listener
  void onStatusChange(StatusChangeHandler handler) {
    _notificationHandlers.onStatusChange(handler);
  }

  /// Register a success response handler
  void onSuccess(ResponseHandler handler) {
    _responseHandlers.onSuccess(handler);
  }

  /// Register an error response handler
  void onError(ResponseHandler handler) {
    _responseHandlers.onError(handler);
  }

  /// Register handler for invalid format errors
  void onInvalidFormat(ResponseHandler handler) {
    _responseHandlers.onInvalidFormat(handler);
  }

  /// Register handler for internal errors
  void onInternalError(ResponseHandler handler) {
    _responseHandlers.onInternalError(handler);
  }

  /// Clear all handlers
  void clearAllHandlers() {
    _responseHandlers.clear();
    _notificationHandlers.clear();
  }
}
