import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/screens/auth/login_screen.dart';
import 'package:driver_app/screens/auth/register_screen.dart';
import 'package:driver_app/services/api_client.dart';

class _FakeApi extends ApiClient {
  _FakeApi() : super(dio: Dio(BaseOptions()));
  bool failLogin = false;
  bool failRegister = false;
  Map<String, dynamic> loginData = {'user_type': 'driver', 'user_id': 'd-1', 'access_token': 't', 'refresh_token': 'r'};

  @override
  Future<Response> post(String path, {dynamic data, Map<String, dynamic>? headers}) async {
    if (path.contains('/auth/login') && failLogin) {
      throw DioException(requestOptions: RequestOptions(path: path));
    }
    if (path.contains('/auth/register') && failRegister) {
      throw DioException(requestOptions: RequestOptions(path: path));
    }
    return Response(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data: {'data': loginData},
    );
  }
}

class _FakeStorage extends FlutterSecureStorage {
  _FakeStorage();
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
  }) async {}
}

AuthNotifier _auth(_FakeApi api) => AuthNotifier(api, _FakeStorage());

Widget _login(_FakeApi api) {
  return ProviderScope(
    overrides: [authProvider.overrideWith((_) => _auth(api))],
    child: const MaterialApp(home: LoginScreen()),
  );
}

void main() {
  group('LoginScreen', () {
    testWidgets('shows title, fields and buttons', (tester) async {
      await tester.pumpWidget(_login(_FakeApi()));
      await tester.pump();
      expect(find.text('G-Flow Driver'), findsOneWidget);
      expect(find.text('Email'), findsOneWidget);
      expect(find.text('Password'), findsOneWidget);
      expect(find.text('Masuk'), findsOneWidget);
      expect(find.text('Belum punya akun? Daftar driver'), findsOneWidget);
    });

    testWidgets('validates invalid email and short password', (tester) async {
      await tester.pumpWidget(_login(_FakeApi()));
      await tester.pump();
      await tester.enterText(find.byType(TextFormField).at(0), 'not-an-email');
      await tester.tap(find.text('Masuk'));
      await tester.pumpAndSettle();
      expect(find.text('Email tidak valid'), findsOneWidget);
      expect(find.text('Minimal 6 karakter'), findsOneWidget);
    });

    testWidgets('toggles password visibility', (tester) async {
      await tester.pumpWidget(_login(_FakeApi()));
      await tester.pump();
      expect(find.byIcon(Icons.visibility_off), findsOneWidget);
      await tester.tap(find.byIcon(Icons.visibility_off));
      await tester.pump();
      expect(find.byIcon(Icons.visibility), findsOneWidget);
    });

    testWidgets('successful login shows success snackbar', (tester) async {
      await tester.pumpWidget(_login(_FakeApi()));
      await tester.pump();
      await tester.enterText(find.byType(TextFormField).at(0), 'a@b.com');
      await tester.enterText(find.byType(TextFormField).at(1), 'secret1');
      await tester.tap(find.text('Masuk'));
      await tester.pumpAndSettle();
      expect(find.text('Login berhasil'), findsOneWidget);
    });

    testWidgets('failed login shows error banner', (tester) async {
      await tester.pumpWidget(_login(_FakeApi()..failLogin = true));
      await tester.pump();
      await tester.enterText(find.byType(TextFormField).at(0), 'a@b.com');
      await tester.enterText(find.byType(TextFormField).at(1), 'secret1');
      await tester.tap(find.text('Masuk'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Login gagal'), findsOneWidget);
    });
  });

  group('RegisterScreen', () {
    Widget register(_FakeApi api) {
      final router = GoRouter(
        initialLocation: '/register',
        routes: [
          GoRoute(path: '/register', builder: (_, __) => const RegisterScreen()),
          GoRoute(path: '/login', builder: (_, __) => const Scaffold(body: Center(child: Text('Login page')))),
        ],
      );
      return ProviderScope(
        overrides: [authProvider.overrideWith((_) => _auth(api))],
        child: MaterialApp.router(routerConfig: router),
      );
    }

    testWidgets('shows all fields and submit button', (tester) async {
      await tester.pumpWidget(register(_FakeApi()));
      await tester.pump();
      expect(find.text('Daftar Driver'), findsOneWidget);
      expect(find.text('Nama lengkap'), findsOneWidget);
      expect(find.text('Email'), findsOneWidget);
      expect(find.text('No. HP'), findsOneWidget);
      expect(find.text('Password'), findsOneWidget);
      expect(find.text('Tipe kendaraan'), findsOneWidget);
      expect(find.text('Daftar'), findsOneWidget);
      expect(find.text('Sudah punya akun? Masuk'), findsOneWidget);
    });

    testWidgets('validates required fields', (tester) async {
      await tester.pumpWidget(register(_FakeApi()));
      await tester.pump();
      await tester.tap(find.text('Daftar'));
      await tester.pumpAndSettle();
      expect(find.text('Nama wajib diisi'), findsOneWidget);
      expect(find.text('Email tidak valid'), findsOneWidget);
      expect(find.text('No. HP tidak valid'), findsOneWidget);
      expect(find.text('Minimal 6 karakter'), findsOneWidget);
    });

    testWidgets('changes vehicle type to Car', (tester) async {
      await tester.pumpWidget(register(_FakeApi()));
      await tester.pump();
      await tester.tap(find.text('Motor'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Mobil').last);
      await tester.pumpAndSettle();
    });

    testWidgets('successful register shows snackbar', (tester) async {
      await tester.pumpWidget(register(_FakeApi()));
      await tester.pump();
      await tester.enterText(find.byType(TextFormField).at(0), 'Nama');
      await tester.enterText(find.byType(TextFormField).at(1), 'a@b.com');
      await tester.enterText(find.byType(TextFormField).at(2), '0812345678');
      await tester.enterText(find.byType(TextFormField).at(3), 'secret1');
      await tester.enterText(find.byType(TextFormField).at(4), 'B123');
      await tester.tap(find.text('Daftar'));
      await tester.pumpAndSettle();
      expect(find.text('Registrasi berhasil, silakan login'), findsOneWidget);
    });

    testWidgets('failed register shows error', (tester) async {
      await tester.pumpWidget(register(_FakeApi()..failRegister = true));
      await tester.pump();
      await tester.enterText(find.byType(TextFormField).at(0), 'Nama');
      await tester.enterText(find.byType(TextFormField).at(1), 'a@b.com');
      await tester.enterText(find.byType(TextFormField).at(2), '0812345678');
      await tester.enterText(find.byType(TextFormField).at(3), 'secret1');
      await tester.tap(find.text('Daftar'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Registrasi akun gagal'), findsOneWidget);
    });
  });
}


