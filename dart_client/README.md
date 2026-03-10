# Dart RPC Client

A high-performance, modular Dart client for the JSON-RPC Process Manager. Built using only Dart standard libraries with no external dependencies.

## Features

✅ **Class-Based Architecture**
- Clean, modular design following Dart best practices
- Type-safe message handling
- Comprehensive error handling

✅ **Response Handler Registry**
- Register handlers for specific response types
- Support for `ResponseTypeSuccess` and `ResponseTypeError`
- Dedicated handlers for error types:
  - `ErrorTypeInvalidFormat`
  - `ErrorTypeInternalError`
  - `ErrorTypeConversionError`

✅ **Real-Time Notification Listeners**
- Dedicated log listener for process stdout/stderr
- Status change listener for process state transitions
- Generic notification handler for custom types

✅ **Async/Await Support**
- All requests return `Future<Response>`
- Non-blocking I/O operations
- Clean async API

✅ **Command Methods**
- `startProcess()` - Launch new processes
- `stopProcess()` - Terminate processes
- `getProcessStatus()` - Query specific process
- `getAllStatuses()` - Query all processes
- `getProcessLogs()` - Retrieve process logs

✅ **Event Methods**
- `sendEvent()` - Send custom events
- `shutdown()` - Graceful shutdown
- `healthCheck()` - Server health check
- `downloadFile()` - File download event

✅ **Performance Optimized**
- Efficient buffer management
- Stream-based socket communication
- Minimal memory footprint
- No external dependencies

## Installation

Since this uses only Dart standard libraries, simply copy the `lib` folder to your project:

```
your_project/
  lib/
    rpc_client/
      rpc_client.dart
      types.dart
      handlers.dart
```

Or include it as a local package in `pubspec.yaml`:

```yaml
dependencies:
  rpc_client:
    path: ../dart_client
```

## Quick Start

### Basic Usage

```dart
import 'package:rpc_client/rpc_client.dart';

void main() async {
  // Create client
  final client = RpcClient(host: 'localhost', port: 8080);
  
  // Connect
  await client.connect();
  
  // Register notification listeners
  client.onLog((log) {
    print('[${log.processName}] ${log.log}');
  });
  
  client.onStatusChange((status) {
    print('${status.name} -> ${status.status}');
  });
  
  // Start a process
  final process = ProcessRequest(
    name: 'my-app',
    command: 'python',
    args: ['-u', 'app.py'],
  );
  
  final response = await client.startProcess(process);
  
  if (response.isSuccess) {
    print('Process started!');
  }
  
  // Clean up
  await client.disconnect();
}
```

### Response Handler Registry

```dart
// Register handler for all success responses
client.onSuccess((response) {
  print('Success: ${response.successData?.message}');
});

// Register handler for all errors
client.onError((response) {
  print('Error: ${response.error?.message}');
});

// Register handler for specific error types
client.onInvalidFormat((response) {
  print('Invalid format error occurred');
});

client.onInternalError((response) {
  print('Server internal error');
});

client.onConversionError((response) {
  print('Conversion error');
});

// Or use the registry directly for custom types
client.responseHandlers.register('custom_type', (response) {
  // Handle custom response type
});
```

### Notification Listeners

```dart
// Listen for process logs
client.notificationHandlers.onLog((log) {
  if (log.isStderr) {
    stderr.writeln('[ERROR] ${log.processName}: ${log.log}');
  } else {
    print('[LOG] ${log.processName}: ${log.log}');
  }
});

// Listen for status changes
client.notificationHandlers.onStatusChange((status) {
  print('Process ${status.name}:');
  print('  Status: ${status.status}');
  print('  PID: ${status.pid}');
  
  if (status.isStopped && status.exitCode != null) {
    print('  Exit code: ${status.exitCode}');
  }
});

// Listen for all notifications
client.notificationHandlers.onNotification((notification) {
  print('Received: ${notification.params.type}');
});
```

### Process Management

```dart
// Start a process
final startResponse = await client.startProcess(
  ProcessRequest(
    name: 'worker',
    command: 'node',
    args: ['worker.js'],
    workDir: '/path/to/project',
    logfilePath: '/var/log/worker.log',
  ),
);

// Get process status
final statusResponse = await client.getProcessStatus('worker');
if (statusResponse.isSuccess) {
  final data = statusResponse.successData?.data;
  final status = ProcessStatusData.fromJson(data);
  print('PID: ${status.pid}, Status: ${status.status}');
}

// Get all process statuses
final allStatusResponse = await client.getAllStatuses();

// Get process logs
final logsResponse = await client.getProcessLogs('worker');

// Stop a process
final stopResponse = await client.stopProcess('worker');
```

### Custom Events

```dart
// Send custom event
final response = await client.sendEvent('my_event', data: {
  'key': 'value',
  'timestamp': DateTime.now().millisecondsSinceEpoch,
});

// Predefined events
await client.healthCheck();
await client.shutdown(data: {'timeout': 30});
await client.downloadFile('https://example.com/file', '/tmp/file');
```

## Architecture

### Class Structure

```
RpcClient
├── ResponseHandlerRegistry
│   ├── onSuccess()
│   ├── onError()
│   ├── onErrorType()
│   └── dispatch()
├── NotificationListenerRegistry
│   ├── onLog()
│   ├── onStatusChange()
│   ├── onNotification()
│   └── dispatch()
└── Command/Event Methods
    ├── startProcess()
    ├── stopProcess()
    ├── getProcessStatus()
    ├── getAllStatuses()
    ├── getProcessLogs()
    └── sendEvent()
```

### Message Flow

```
Client                        Server
  │                             │
  ├──── Command/Event ─────────>│
  │                             │
  │<──── Response ──────────────┤
  │                             │
  │<──── Notification ──────────┤ (async, no response)
  │<──── Notification ──────────┤
  │                             │
```

## Type Safety

All types are strongly typed with proper null safety:

```dart
// Response is always non-null
Response response = await client.startProcess(request);

// Check response type
if (response.isSuccess) {
  CommandResponseData? data = response.successData;
  print(data?.message);
}

if (response.isError) {
  ErrorResponse? error = response.error;
  print('${error?.type}: ${error?.message}');
}

// Process status with null safety
ProcessStatusData status = ProcessStatusData.fromJson(json);
print('Exit code: ${status.exitCode ?? 'still running'}');
```

## Performance Considerations

1. **Stream Processing**: Uses Dart streams for efficient data handling
2. **Buffer Management**: Minimal buffer allocation with StringBuffer
3. **Async I/O**: Non-blocking socket operations
4. **Memory Efficient**: No external dependencies, small memory footprint
5. **Handler Copy**: Handlers are copied before iteration to prevent concurrent modification

## Error Handling

```dart
try {
  await client.connect();
  
  final response = await client.startProcess(request);
  
  if (response.isError) {
    final error = response.error!;
    
    switch (error.type) {
      case ErrorType.invalidFormat:
        print('Invalid message format');
        break;
      case ErrorType.internalError:
        print('Server error: ${error.message}');
        break;
      case ErrorType.conversionError:
        print('Type conversion failed');
        break;
    }
  }
} catch (e) {
  print('Connection error: $e');
} finally {
  await client.disconnect();
}
```

## Best Practices

1. **Always disconnect**: Call `disconnect()` in a finally block
2. **Check response types**: Use `isSuccess` and `isError` properties
3. **Handle notifications**: Register handlers before connecting
4. **Error handling**: Wrap operations in try-catch blocks
5. **Resource cleanup**: Clear handlers when done with `clearAllHandlers()`

## Example Output

```
Connecting to RPC server...
Connected successfully!
============================================================

1️⃣  Starting a process...
------------------------------------------------------------
🟡 [STATUS] test-echo -> transitioning
✅ Process 'test-echo' started successfully
🟢 [STATUS] test-echo -> running
📝 [LOG] test-echo (stdout): Line 1
📝 [LOG] test-echo (stdout): Line 2
📝 [LOG] test-echo (stdout): Line 3

2️⃣  Getting process status...
------------------------------------------------------------
✅ Status for process 'test-echo' retrieved
   Name: test-echo
   PID: 12345
   Status: running
   Start Time: 2026-03-10 10:30:00.000

...
```

## Development

Run the example:
```bash
cd example
dart run example.dart
```

Run with analysis:
```bash
dart analyze
```

## License

See LICENSE file for details.
