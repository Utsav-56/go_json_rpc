/// Message type constants
class MessageType {
  static const String event = 'event';
  static const String command = 'command';
  static const String notification = 'notification';
}

/// Response type constants
class ResponseType {
  static const String success = 'success';
  static const String error = 'error';
}

/// Error type constants
class ErrorType {
  static const String invalidFormat = 'invalid_message_format';
  static const String internalError = 'internal_error';
  static const String conversionError = 'conversion_error';
}

/// Command action constants
class CommandAction {
  static const String start = 'start';
  static const String stop = 'stop';
  static const String getStatus = 'get_status';
  static const String getLogs = 'get_logs';
}

/// Notification type constants
class NotificationType {
  static const String log = 'log';
  static const String statusChange = 'status_change';
}

/// Process status constants
class ProcessStatus {
  static const String transitioning = 'transitioning';
  static const String running = 'running';
  static const String stopped = 'stopped';
}

/// Base message class
class Message {
  final String? id;
  final String type;
  final Map<String, dynamic>? params;

  Message({
    this.id,
    required this.type,
    this.params,
  });

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{
      'type': type,
    };
    
    if (id != null) {
      json['id'] = id;
    }
    
    if (params != null) {
      json['params'] = params;
    }
    
    return json;
  }

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] as String?,
      type: json['type'] as String,
      params: json['params'] as Map<String, dynamic>?,
    );
  }
}

/// Response message
class Response {
  final String id;
  final String type;
  final dynamic result;

  Response({
    required this.id,
    required this.type,
    this.result,
  });

  factory Response.fromJson(Map<String, dynamic> json) {
    return Response(
      id: json['id'] as String,
      type: json['type'] as String,
      result: json['result'],
    );
  }

  bool get isSuccess => type == ResponseType.success;
  bool get isError => type == ResponseType.error;

  ErrorResponse? get error {
    if (isError && result is Map<String, dynamic>) {
      return ErrorResponse.fromJson(result as Map<String, dynamic>);
    }
    return null;
  }

  CommandResponseData? get successData {
    if (isSuccess && result is Map<String, dynamic>) {
      return CommandResponseData.fromJson(result as Map<String, dynamic>);
    }
    return null;
  }
}

/// Error response
class ErrorResponse {
  final String type;
  final String message;
  final int timestamp;

  ErrorResponse({
    required this.type,
    required this.message,
    required this.timestamp,
  });

  factory ErrorResponse.fromJson(Map<String, dynamic> json) {
    return ErrorResponse(
      type: json['type'] as String,
      message: json['message'] as String,
      timestamp: json['timestamp'] as int,
    );
  }

  @override
  String toString() => 'ErrorResponse(type: $type, message: $message)';
}

/// Command response data
class CommandResponseData {
  final String message;
  final dynamic data;

  CommandResponseData({
    required this.message,
    this.data,
  });

  factory CommandResponseData.fromJson(Map<String, dynamic> json) {
    return CommandResponseData(
      message: json['message'] as String,
      data: json['data'],
    );
  }
}

/// Notification message
class NotificationMessage {
  final String type;
  final NotificationParams params;

  NotificationMessage({
    required this.type,
    required this.params,
  });

  factory NotificationMessage.fromJson(Map<String, dynamic> json) {
    return NotificationMessage(
      type: json['type'] as String,
      params: NotificationParams.fromJson(json['params'] as Map<String, dynamic>),
    );
  }
}

/// Notification parameters
class NotificationParams {
  final String type;
  final Map<String, dynamic> data;

  NotificationParams({
    required this.type,
    required this.data,
  });

  factory NotificationParams.fromJson(Map<String, dynamic> json) {
    return NotificationParams(
      type: json['type'] as String,
      data: json['data'] as Map<String, dynamic>,
    );
  }

  bool get isLog => type == NotificationType.log;
  bool get isStatusChange => type == NotificationType.statusChange;

  ProcessLogData? get logData {
    if (isLog) {
      return ProcessLogData.fromJson(data);
    }
    return null;
  }

  ProcessStatusData? get statusData {
    if (isStatusChange) {
      return ProcessStatusData.fromJson(data);
    }
    return null;
  }
}

/// Process log data
class ProcessLogData {
  final String type;
  final String processName;
  final String log;
  final int timestamp;

  ProcessLogData({
    required this.type,
    required this.processName,
    required this.log,
    required this.timestamp,
  });

  factory ProcessLogData.fromJson(Map<String, dynamic> json) {
    return ProcessLogData(
      type: json['type'] as String,
      processName: json['process_name'] as String,
      log: json['log'] as String,
      timestamp: json['timestamp'] as int,
    );
  }

  bool get isStdout => type == 'stdout';
  bool get isStderr => type == 'stderr';

  @override
  String toString() => '[$processName:$type] $log';
}

/// Process status data
class ProcessStatusData {
  final String name;
  final int pid;
  final int startTime;
  final String status;
  final int? exitCode;

  ProcessStatusData({
    required this.name,
    required this.pid,
    required this.startTime,
    required this.status,
    this.exitCode,
  });

  factory ProcessStatusData.fromJson(Map<String, dynamic> json) {
    return ProcessStatusData(
      name: json['name'] as String,
      pid: json['pid'] as int,
      startTime: json['start_time'] as int,
      status: json['status'] as String,
      exitCode: json['exit_code'] as int?,
    );
  }

  bool get isTransitioning => status == ProcessStatus.transitioning;
  bool get isRunning => status == ProcessStatus.running;
  bool get isStopped => status == ProcessStatus.stopped;

  @override
  String toString() => 'Process($name, status: $status, pid: $pid)';
}

/// Process request for starting processes
class ProcessRequest {
  final String name;
  final String command;
  final List<String>? args;
  final String? workDir;
  final String? logfilePath;

  ProcessRequest({
    required this.name,
    required this.command,
    this.args,
    this.workDir,
    this.logfilePath,
  });

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{
      'name': name,
      'command': command,
    };

    if (args != null && args!.isNotEmpty) {
      json['args'] = args;
    }

    if (workDir != null) {
      json['work_dir'] = workDir;
    }

    if (logfilePath != null) {
      json['logfile_path'] = logfilePath;
    }

    return json;
  }
}

/// Event parameters
class EventParams {
  final String name;
  final dynamic data;

  EventParams({
    required this.name,
    this.data,
  });

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{
      'name': name,
    };

    if (data != null) {
      json['data'] = data;
    }

    return json;
  }
}

/// Command parameters
class CommandParams {
  final String action;
  final ProcessRequest? process;
  final String? name;

  CommandParams({
    required this.action,
    this.process,
    this.name,
  });

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{
      'action': action,
    };

    if (process != null) {
      json['process'] = process!.toJson();
    }

    if (name != null) {
      json['name'] = name;
    }

    return json;
  }
}
