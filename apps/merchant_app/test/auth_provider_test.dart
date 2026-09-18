import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:merchant_app/providers/auth_provider.dart';
import 'package:merchant_app/services/api_client.dart';

class FakeDio implements HttpClientAdapter {
  final _requests = <(String, String, Map<String, dynamic>?)>[];
  final List<Response> responses;
  final List<Object> errors;

  FakeDio({required this.responses, this.errors = const []});

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    _requests.add((options.method, options.path, options.data as Map<String, dynamic>?));
    if (errors.isNotEmpty) {
      throw errors.removeAt(0);
    }
    if (responses.isEmpty) {
      return ResponseBody.fromString('{}', 200,
          headers: const {Headers.contentTypeHeader: [Headers.jsonContentType]});
    }
    final r = responses.removeAt(0);
    final data = r.data;
    final body = data is String ? data : _encode(data);
    return ResponseBody.fromString(body, r.statusCode ?? 200,
        headers: const {Headers.contentTypeHeader: [Headers.jsonContentType]});
  }

  @override
  void close({bool force = false}) {}

  List<(String, String, Map<String, dynamic>?)> get requests => List.of(_requests);
}

String _encode(Object? o) {
  if (o is String) return '"${o.replaceAll('"', '\\"')}"';
  if (o is Map) return '{${o.entries.map((e) => '"${e.key}":${_encode(e.value)}').join(',')}}';
  if (o is List) return '[${o.map(_encode).join(',')}]';
  if (o is bool) return o.toString();
  if (o is num) return o.toString();
  return 'null';
}

class FakeStorage extends Fake implements FlutterSecureStorage {
  final Map<String, String> store = <String, String>{};

  @override
  Future<void> write({required String key, required String? value, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async {
    store[key] = value ?? '';
  }

  @override
  Future<String?> read({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async {
    return store[key];
  }

  @override
  Future<void> delete({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async {
    store.remove(key);
  }
}

ApiClient buildApi(List<Response> responses, {List<Object> errors = const []}) {
  final dio = Dio(BaseOptions());
  dio.httpClientAdapter = FakeDio(responses: responses, errors: errors);
  return ApiClient(dio: dio, storage: FakeStorage());
}

void main() {
  group('AuthNotifier', () {
    test('login success sets authenticated state and stores tokens', () async {
      final api = buildApi([
        Response(
          requestOptions: RequestOptions(path: '/api/v1/auth/login'),
          statusCode: 200,
          data: {
            'data': {
              'user_type': 'merchant',
              'access_token': 'at',
              'refresh_token': 'rt',
              'email': 'm@m.com',
            }
          },
        ),
      ]);
      final storage = FakeStorage()..store['merchant_id'] = 'm1';
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.login('m@m.com', 'pass');

      expect(ok, isTrue);
      expect(notifier.state.isAuthenticated, isTrue);
      expect(notifier.state.userType, 'merchant');
      expect(storage.store['access_token'], 'at');
      expect(storage.store['refresh_token'], 'rt');
    });

    test('login rejects non-merchant account', () async {
      final api = buildApi([
        Response(
          requestOptions: RequestOptions(path: '/x'),
          statusCode: 200,
          data: {'data': {'user_type': 'customer'}},
        ),
      ]);
      final storage = FakeStorage();
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.login('c@c.com', 'pass');

      expect(ok, isFalse);
      expect(notifier.state.isAuthenticated, isFalse);
      expect(notifier.state.error, 'Akun ini bukan akun merchant');
    });

    test('login handles DioException error', () async {
      final api = buildApi([], errors: [
        DioException(requestOptions: RequestOptions(path: '/x'), message: 'network down'),
      ]);
      final storage = FakeStorage();
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.login('m@m.com', 'pass');

      expect(ok, isFalse);
      expect(notifier.state.error, contains('Login gagal'));
    });

    test('login handles generic error', () async {
      final api = buildApi([], errors: [StateError('boom')]);
      final storage = FakeStorage();
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.login('m@m.com', 'pass');

      expect(ok, isFalse);
      expect(notifier.state.error, contains('Login gagal'));
    });

    test('registerUser success', () async {
      final api = buildApi([
        Response(requestOptions: RequestOptions(path: '/x'), statusCode: 200, data: {'data': {}}),
      ]);
      final storage = FakeStorage();
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.registerUser(email: 'm@m.com', password: 'pass123', name: 'Me', phone: '123');

      expect(ok, isTrue);
      expect(notifier.state.isLoading, isFalse);
    });

    test('registerUser failure', () async {
      final api = buildApi([], errors: [DioException(requestOptions: RequestOptions(path: '/x'), message: 'fail')]);
      final storage = FakeStorage();
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.registerUser(email: 'm@m.com', password: 'pass123', name: 'Me', phone: '123');

      expect(ok, isFalse);
      expect(notifier.state.error, contains('Registrasi akun gagal'));
    });

    test('registerMerchant success stores merchant id and authenticates', () async {
      final api = buildApi([
        Response(
          requestOptions: RequestOptions(path: '/x'),
          statusCode: 200,
          data: {'data': {'id': 'merch-1', 'merchant_name': 'Toko'}},
        ),
      ]);
      final storage = FakeStorage();
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.registerMerchant({'merchant_name': 'Toko'});

      expect(ok, isTrue);
      expect(notifier.state.isAuthenticated, isTrue);
      expect(notifier.state.userType, 'merchant');
      expect(storage.store['merchant_id'], 'merch-1');
    });

    test('registerMerchant failure', () async {
      final api = buildApi([], errors: [StateError('x')]);
      final storage = FakeStorage();
      final notifier = AuthNotifier(api, storage);

      final ok = await notifier.registerMerchant({'merchant_name': 'Toko'});

      expect(ok, isFalse);
      expect(notifier.state.error, contains('Registrasi merchant gagal'));
    });

    test('logout clears storage and resets state', () async {
      final api = buildApi([]);
      final storage = FakeStorage()
        ..store['access_token'] = 'at'
        ..store['refresh_token'] = 'rt'
        ..store['merchant_id'] = 'm1';
      final notifier = AuthNotifier(api, storage)..state = const AuthState(isAuthenticated: true);

      await notifier.logout();

      expect(storage.store.keys, isEmpty);
      expect(notifier.state.isAuthenticated, isFalse);
    });
  });

  group('AuthState', () {
    test('copyWith updates fields', () {
      const s = AuthState();
      final c = s.copyWith(isLoading: true, user: {'a': 1}, isAuthenticated: true, userType: 'merchant', error: 'err');
      expect(c.isLoading, isTrue);
      expect(c.isAuthenticated, isTrue);
      expect(c.userType, 'merchant');
      expect(c.error, 'err');
    });
  });

  group('apiErrorMessage', () {
    test('extracts message from DioException response data', () {
      final e = DioException(
        requestOptions: RequestOptions(path: '/x'),
        response: Response(
          requestOptions: RequestOptions(path: '/x'),
          statusCode: 400,
          data: {'message': 'Bad request'},
        ),
      );
      expect(apiErrorMessage(e), 'Bad request');
    });

    test('uses message when no response data', () {
      final e = DioException(requestOptions: RequestOptions(path: '/x'), message: 'plain');
      expect(apiErrorMessage(e), 'plain');
    });

    test('falls back to toString for non-dio errors', () {
      expect(apiErrorMessage(StateError('oops')), 'Bad state: oops');
    });
  });
}
