import 'dart:async';
import 'dart:io';

import '../lib/rpc_client.dart';
import '../lib/types.dart';

void main() async {
  // Create RPC client
  final client = RpcClient(host: 'localhost', port: 8080);

  print('Connecting to RPC server...');

  try {
    await client.connect();
    print('Connected successfully!');
    print('=' * 60);

    // Register notification handlers
    _registerNotificationHandlers(client);

    // Register response handlers
    _registerResponseHandlers(client);

    // Give handlers time to register
    await Future.delayed(Duration(milliseconds: 100));

    // Example 1: Start a process
    await _startProcessExample(client);
    await Future.delayed(Duration(seconds: 3));

    // Example 2: Get process status
    await _getStatusExample(client);
    await Future.delayed(Duration(seconds: 1));

    // Example 3: Send custom events
    await _customEventExamples(client);
    await Future.delayed(Duration(seconds: 1));

    // Example 4: Get all statuses
    await _getAllStatusesExample(client);
    await Future.delayed(Duration(seconds: 2));

    // Example 5: Stop process
    await _stopProcessExample(client);
    await Future.delayed(Duration(seconds: 2));

    // Example 6: Get logs
    await _getLogsExample(client);

    // Keep connection alive to receive final notifications
    print('\n${'=' * 60}');
    print('Waiting for final notifications...');
    await Future.delayed(Duration(seconds: 3));
  } catch (e) {
    print('Error: $e');
  } finally {
    await client.disconnect();
    print('\nDisconnected from server');
    exit(0);
  }
}

void _registerNotificationHandlers(RpcClient client) {
  // Register log handler
  client.onLog((log) {
    print(' [LOG] ${log.processName} (${log.type}): ${log.log}');
  });

  // Register status change handler
  client.onStatusChange((status) {
    print('[STATUS] ${status.name} -> ${status.status}');
    if (status.exitCode != null) {
      print('   Exit code: ${status.exitCode}');
    }
  });
}

void _registerResponseHandlers(RpcClient client) {
  // Register success handler
  client.onSuccess((response) {
    // This will catch all success responses
    // You can add custom logic here if needed
  });

  // Register error handlers
  client.onError((response) {
    final error = response.error;
    print('[ERROR] ${error?.type}: ${error?.message}');
  });

  // Register specific error type handlers
  client.onInvalidFormat((response) {
    print('[INVALID FORMAT] Invalid message format detected');
  });

  client.onInternalError((response) {
    print('[INTERNAL ERROR] Server internal error occurred');
  });
}

Future<void> _startProcessExample(RpcClient client) async {
  print('\n Starting a process...');
  print('-' * 60);

  final processRequest = ProcessRequest(
    name: 'test-echo',
    command: 'bash',
    args: ['-c', 'for i in {1..5}; do echo "Line \$i"; sleep 1; done'],
  );

  try {
    final response = await client.startProcess(processRequest);

    if (response.isSuccess) {
      print('${response.successData?.message}');
    } else {
      print('Failed: ${response.error?.message}');
    }
  } catch (e) {
    print('Error: $e');
  }
}

Future<void> _getStatusExample(RpcClient client) async {
  print('\n Getting process status...');
  print('-' * 60);

  try {
    final response = await client.getProcessStatus('test-echo');

    if (response.isSuccess) {
      print('${response.successData?.message}');

      final data = response.successData?.data;
      if (data is Map<String, dynamic>) {
        final status = ProcessStatusData.fromJson(data);
        print('   Name: ${status.name}');
        print('   PID: ${status.pid}');
        print('   Status: ${status.status}');
        print('   Start Time: ${DateTime.fromMillisecondsSinceEpoch(status.startTime)}');
      }
    } else {
      print('Failed: ${response.error?.message}');
    }
  } catch (e) {
    print('Error: $e');
  }
}

Future<void> _customEventExamples(RpcClient client) async {
  print('\n  Sending custom events...');
  print('-' * 60);

  // Health check event
  try {
    final response = await client.healthCheck();
    if (response.isSuccess) {
      print('Health Check: ${response.successData?.message}');
      print('   Data: ${response.successData?.data}');
    }
  } catch (e) {
    print('Health Check Error: $e');
  }

  // Custom event
  try {
    final response = await client.sendEvent('custom_event', data: {
      'key': 'value',
      'timestamp': DateTime.now().millisecondsSinceEpoch,
    });

    if (response.isSuccess) {
      print('Custom Event: ${response.successData?.message}');
    } else {
      print('Custom Event: ${response.error?.message}');
    }
  } catch (e) {
    print('Custom Event Error: $e');
  }
}

Future<void> _getAllStatusesExample(RpcClient client) async {
  print('\n  Getting all process statuses...');
  print('-' * 60);

  try {
    final response = await client.getAllStatuses();

    if (response.isSuccess) {
      print('${response.successData?.message}');

      final data = response.successData?.data;
      if (data is Map<String, dynamic>) {
        for (final entry in data.entries) {
          final processData = entry.value as Map<String, dynamic>;
          final status = ProcessStatusData.fromJson(processData);
          print('   • ${status.name}: ${status.status} (PID: ${status.pid})');
        }
      }
    } else {
      print('Failed: ${response.error?.message}');
    }
  } catch (e) {
    print('Error: $e');
  }
}

Future<void> _stopProcessExample(RpcClient client) async {
  print('\n Stopping process...');
  print('-' * 60);

  try {
    final response = await client.stopProcess('test-echo');

    if (response.isSuccess) {
      print('${response.successData?.message}');
    } else {
      print('Failed: ${response.error?.message}');
    }
  } catch (e) {
    print('Error: $e');
  }
}

Future<void> _getLogsExample(RpcClient client) async {
  print('\n  Getting process logs...');
  print('-' * 60);

  try {
    final response = await client.getProcessLogs('test-echo');

    if (response.isSuccess) {
      print('${response.successData?.message}');

      final logs = response.successData?.data;
      if (logs is List) {
        print('   Retrieved ${logs.length} log lines:');
        for (final log in logs.take(5)) {
          print('   • $log');
        }
        if (logs.length > 5) {
          print('   ... and ${logs.length - 5} more lines');
        }
      }
    } else {
      print('Failed: ${response.error?.message}');
    }
  } catch (e) {
    print('Error: $e');
  }
}
