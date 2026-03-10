import 'types.dart';

/// Callback type for response handlers
typedef ResponseHandler = void Function(Response response);

/// Callback type for log notifications
typedef LogHandler = void Function(ProcessLogData log);

/// Callback type for status change notifications
typedef StatusChangeHandler = void Function(ProcessStatusData status);

/// Callback type for generic notification handler
typedef NotificationHandler = void Function(NotificationMessage notification);

/// Response handler registry for different response types
class ResponseHandlerRegistry {
  final Map<String, List<ResponseHandler>> _handlers = {};

  /// Register a handler for a specific response type
  void register(String responseType, ResponseHandler handler) {
    _handlers.putIfAbsent(responseType, () => []).add(handler);
  }

  /// Unregister a handler for a specific response type
  void unregister(String responseType, ResponseHandler handler) {
    _handlers[responseType]?.remove(handler);
    if (_handlers[responseType]?.isEmpty ?? false) {
      _handlers.remove(responseType);
    }
  }

  /// Register handler for success responses
  void onSuccess(ResponseHandler handler) {
    register(ResponseType.success, handler);
  }

  /// Register handler for error responses
  void onError(ResponseHandler handler) {
    register(ResponseType.error, handler);
  }

  /// Register handler for specific error types
  void onErrorType(String errorType, ResponseHandler handler) {
    register('error:$errorType', handler);
  }

  /// Register handler for invalid format errors
  void onInvalidFormat(ResponseHandler handler) {
    onErrorType(ErrorType.invalidFormat, handler);
  }

  /// Register handler for internal errors
  void onInternalError(ResponseHandler handler) {
    onErrorType(ErrorType.internalError, handler);
  }

  /// Register handler for conversion errors
  void onConversionError(ResponseHandler handler) {
    onErrorType(ErrorType.conversionError, handler);
  }

  /// Dispatch a response to registered handlers
  void dispatch(Response response) {
    // Call general response type handlers
    final typeHandlers = _handlers[response.type];
    if (typeHandlers != null) {
      for (final handler in List.from(typeHandlers)) {
        handler(response);
      }
    }

    // Call specific error type handlers
    if (response.isError) {
      final error = response.error;
      if (error != null) {
        final errorHandlers = _handlers['error:${error.type}'];
        if (errorHandlers != null) {
          for (final handler in List.from(errorHandlers)) {
            handler(response);
          }
        }
      }
    }
  }

  /// Clear all handlers
  void clear() {
    _handlers.clear();
  }

  /// Clear handlers for a specific type
  void clearType(String responseType) {
    _handlers.remove(responseType);
  }
}

/// Notification listener registry
class NotificationListenerRegistry {
  final List<LogHandler> _logHandlers = [];
  final List<StatusChangeHandler> _statusHandlers = [];
  final List<NotificationHandler> _genericHandlers = [];

  /// Register a log handler
  void onLog(LogHandler handler) {
    _logHandlers.add(handler);
  }

  /// Register a status change handler
  void onStatusChange(StatusChangeHandler handler) {
    _statusHandlers.add(handler);
  }

  /// Register a generic notification handler
  void onNotification(NotificationHandler handler) {
    _genericHandlers.add(handler);
  }

  /// Remove a log handler
  void removeLogHandler(LogHandler handler) {
    _logHandlers.remove(handler);
  }

  /// Remove a status change handler
  void removeStatusHandler(StatusChangeHandler handler) {
    _statusHandlers.remove(handler);
  }

  /// Remove a generic notification handler
  void removeNotificationHandler(NotificationHandler handler) {
    _genericHandlers.remove(handler);
  }

  /// Dispatch a notification to registered handlers
  void dispatch(NotificationMessage notification) {
    // Call generic handlers
    for (final handler in List.from(_genericHandlers)) {
      handler(notification);
    }

    // Call specific handlers based on type
    if (notification.params.isLog) {
      final logData = notification.params.logData;
      if (logData != null) {
        for (final handler in List.from(_logHandlers)) {
          handler(logData);
        }
      }
    } else if (notification.params.isStatusChange) {
      final statusData = notification.params.statusData;
      if (statusData != null) {
        for (final handler in List.from(_statusHandlers)) {
          handler(statusData);
        }
      }
    }
  }

  /// Clear all handlers
  void clear() {
    _logHandlers.clear();
    _statusHandlers.clear();
    _genericHandlers.clear();
  }
}
