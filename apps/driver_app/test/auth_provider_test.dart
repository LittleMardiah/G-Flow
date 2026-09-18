import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/services/api_client.dart';

class _FakeStorage extends FlutterSecureStorage {
  final Map<String, String> store = {};

  @override
  Future<void> write({
    required String key,
    required String? value,
    IOSOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    MacOsOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    if (value != null) store[key] = value;
  }

  @override
  Future<String?> read({
    required String key,
    IOSOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    MacOsOptions? mOptions,
    WindowsOptions? wOptions,
  }) async =>
      store[key];

  @override
  Future<void> delete({
    required String key,
    IOSOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    MacOsOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    store.remove(key);
  }
}

class _FakeApi extends ApiClient {
  _FakeApi() : super(dio: Dio(BaseOptions()));
  bool fail = false;
  Map<String, dynamic> loginData = {
    'user_type': 'driver',
    'user_id': 'd-1',
    'access_token': 'tok',
    'refresh_token': 'rtok',
  };
  final List<String> posts = [];

  @override
  Future<Response> post(String path, {dynamic data, Map<String, dynamic>? headers}) async {
    posts.add(path);
    if (fail) throw DioException(requestOptions: RequestOptions(path: path));
    return Response(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data: {'data': loginData},
    );
  }
}

void main() {
  group('AuthState', () {
    test('copyWith updates fields', () {
      const s = AuthState(isLoading: true, error: 'e');
      final next = s.copyWith(isLoading: false, user: {'a': 1}, isAuthenticated: true);
      expect(next.isLoading, isFalse);
      expect(next.user, {'a': 1});
      expect(next.isAuthenticated, isTrue);
      expect(next.error, 'e');
    });
  });

  group('AuthNotifier.login', () {
    test('successful driver login authenticates and stores tokens', () async {
      final api = _FakeApi();
      final storage = _FakeStorage();
      final notifier = AuthNotifier(api, storage);
      final ok = await notifier.login('a@b.com', 'secret');
      expect(ok, isTrue);
      final state = notifier.state;
      expect(state.isAuthenticated, isTrue);
      expect(state.isLoading, isFalse);
      expect(state.error, isNull);
      expect(storage.store['access_token'], 'tok');
      expect(storage.store['refresh_token'], 'rtok');
      expect(storage.store[kDriverIdKey], 'd-1');
      expect(storage.store[kUserTypeKey], 'driver');
    });

    test('rejects non-driver account', () async {
      final api = _FakeApi()..loginData = {'user_type': 'customer', 'user_id': 'c-1'};
      final notifier = AuthNotifier(api, _FakeStorage());
      final ok = await notifier.login('a@b.com', 'secret');
      expect(ok, isFalse);
      expect(notifier.state.isAuthenticated, isFalse);
      expect(notifier.state.error, contains('bukan akun driver'));
    });

    test('handles login failure gracefully', () async {
      final api = _FakeApi()..fail = true;
      final notifier = AuthNotifier(api, _FakeStorage());
      final ok = await notifier.login('a@b.com', 'secret');
      expect(ok, isFalse);
      expect(notifier.state.isLoading, isFalse);
      expect(notifier.state.error, contains('Login gagal'));
    });
  });

  group('AuthNotifier.register', () {
    test('successful register returns true', () async {
      final api = _FakeApi();
      final notifier = AuthNotifier(api, _FakeStorage());
      final ok = await notifier.register(
        email: 'a@b.com',
        password: 'secret',
        name: 'A',
        phone: '08123',
        vehicleType: 'MOTORCYCLE',
        vehiclePlate: 'B123',
      );
      expect(ok, isTrue);
      expect(notifier.state.isLoading, isFalse);
      expect(api.posts, contains('/api/v1/auth/register'));
    });

    test('register failure returns false', () async {
      final api = _FakeApi()..fail = true;
      final notifier = AuthNotifier(api, _FakeStorage());
      final ok = await notifier.register(
        email: 'a@b.com',
        password: 'secret',
        name: 'A',
        phone: '08123',
      );
      expect(ok, isFalse);
      expect(notifier.state.error, contains('Registrasi akun gagal'));
    });
  });

  group('AuthNotifier.logout', () {
    test('clears state and storage even on logout failure', () async {
      final api = _FakeApi()..fail = true;
      final storage = _FakeStorage()..store['access_token'] = 'tok';
      final notifier = AuthNotifier(api, storage);
      await notifier.logout();
      expect(notifier.state.isAuthenticated, isFalse);
      expect(storage.store.containsKey('access_token'), isFalse);
      expect(storage.store.containsKey(kDriverIdKey), isFalse);
    });
  });
}
