import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/screens/profile/driver_profile_screen.dart';

class _FakeStorage extends FlutterSecureStorage {
  _FakeStorage([Map<String, String>? seed]) : store = seed ?? {};

  final Map<String, String> store;

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
  }) async => store[key];

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

Widget _build(FlutterSecureStorage storage) {
  return ProviderScope(
    overrides: [storageProvider.overrideWithValue(storage)],
    child: const MaterialApp(home: DriverProfileScreen()),
  );
}

void main() {
  testWidgets('render profil dengan data ter-register di perangkat', (
    tester,
  ) async {
    final storage = _FakeStorage({
      kDriverEmailKey: 'driver@mail.com',
      kDriverNameKey: 'Budi Driver',
      kDriverPhoneKey: '081234567891',
      kDriverVehicleTypeKey: 'MOTORCYCLE',
      kDriverVehiclePlateKey: 'B 1234 X',
      kDriverIdKey: 'driver-abc-123',
    });
    await tester.pumpWidget(_build(storage));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Profil Driver'), findsOneWidget);
    expect(find.text('Budi Driver'), findsOneWidget);
    expect(find.text('driver@mail.com'), findsWidgets);
    expect(find.textContaining('ID driver-abc-123'), findsOneWidget);
    expect(find.text('0812****891'), findsOneWidget);
    expect(find.text('MOTORCYCLE'), findsOneWidget);
    expect(find.text('B 1234 X'), findsOneWidget);
    expect(find.text('DRIVER AKTIF'), findsOneWidget);

    await tester.scrollUntilVisible(
      find.text('Logout'),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    expect(find.text('Logout'), findsOneWidget);
  });

  testWidgets('render minimal saat data profil tidak tersedia', (tester) async {
    final storage = _FakeStorage(); // tanpa data mapping
    await tester.pumpWidget(_build(storage));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Driver G-Flow'), findsOneWidget);
    expect(find.text('—'), findsWidgets);
    expect(find.text('Data kendaraan belum tersedia.'), findsOneWidget);

    await tester.scrollUntilVisible(
      find.text('Logout'),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    expect(find.text('Logout'), findsOneWidget);
  });
}
