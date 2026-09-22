import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:customer_app/config/router.dart';
import 'package:customer_app/models/ride_order.dart';
import 'package:customer_app/providers/ride_provider.dart';
import 'package:customer_app/screens/ride/ride_detail_screen.dart';
import 'package:customer_app/screens/ride/ride_history_screen.dart';
import 'package:customer_app/services/api_client.dart';
import 'package:customer_app/services/ride_service.dart';

/// Fake base (pengganti mockito) — pola sama services_test.dart.
abstract class Fake {
  @override
  NoSuchMethodError noSuchMethod(Invocation invocation) =>
      throw UnimplementedError('${invocation.memberName} not implemented');
}

class DioAdapterMock extends Fake implements HttpClientAdapter {
  final Object? data;
  int statusCode = 200;

  DioAdapterMock({this.data, this.statusCode = 200});

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<List<int>>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(
      jsonEncode(data),
      statusCode,
      headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
    );
  }

  @override
  void close({bool force = false}) {}
}

class FakeStorage extends Fake implements FlutterSecureStorage {
  final Map<String, String> store = {};
  @override
  Future<String?> read({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key];
  @override
  Future<void> write({required String key, required String? value, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key] = value ?? '';
  @override
  Future<void> delete({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store.remove(key);
}

RideService makeRideService(DioAdapterMock adapter) =>
    RideService(ApiClient(dio: Dio()..httpClientAdapter = adapter, storage: FakeStorage()));

class _Harness {
  _Harness(this.service) : navigatorKey = GlobalKey<NavigatorState>();

  final GlobalKey<NavigatorState> navigatorKey;
  final RideService service;

  Widget build() {
    return ProviderScope(
      overrides: [rideServiceProvider.overrideWithValue(service)],
      child: MaterialApp(
        navigatorKey: navigatorKey,
        routes: {
          AppRoutes.rideHistory: (_) => const RideHistoryScreen(),
          AppRoutes.rideDetail: (_) => const RideDetailScreen(),
        },
        home: const Scaffold(body: SizedBox()),
      ),
    );
  }
}

Future<void> _push(WidgetTester tester, _Harness h, String route, {Object? args}) async {
  h.navigatorKey.currentState!.pushNamed(route, arguments: args);
  for (var i = 0; i < 10; i++) {
    await tester.pump(const Duration(milliseconds: 20));
  }
}

void main() {
  group('RideHistoryScreen', () {
    testWidgets('render daftar order + status badge + fare aktual', (tester) async {
      final h = _Harness(makeRideService(DioAdapterMock(data: {
        'success': true,
        'data': {
          'orders': [
            {
              'order_id': 'r1',
              'status': 'COMPLETED',
              'pickup_address': 'Jl. Sudirman 99',
              'dropoff_address': 'Jl. Gatot Subroto',
              'estimated_fare': '25000',
              'actual_fare': '24000',
              'created_at': '2026-09-22T10:00:00Z',
            },
            {
              'order_id': 'r2',
              'status': 'CANCELLED',
              'pickup_address': 'Jl. Thamrin',
              'dropoff_address': 'Jl. HR Rasuna Said',
              'estimated_fare': '15000',
              'created_at': '2026-09-21T09:00:00Z',
            },
          ],
        },
        'meta': {'page': 1, 'page_size': 20, 'total': 2, 'total_pages': 1},
      })));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.rideHistory);

      expect(find.text('Riwayat Perjalanan'), findsOneWidget);
      expect(find.text('COMPLETED'), findsOneWidget);
      expect(find.text('CANCELLED'), findsOneWidget);
      expect(find.text('Rp 24.000'), findsOneWidget);
      expect(find.text('Lihat Detail'), findsNWidgets(2));
    });

    testWidgets('empty state menampilkan pesan', (tester) async {
      final h = _Harness(makeRideService(DioAdapterMock(data: {
        'success': true,
        'data': {'orders': <dynamic>[]},
        'meta': {'page': 1, 'page_size': 20, 'total': 0, 'total_pages': 0},
      })));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.rideHistory);

      expect(find.text('Belum ada perjalanan.'), findsOneWidget);
    });
  });

  group('RideDetailScreen', () {
    testWidgets('render detail COMPLETED: driver, fare, waktu, pembayaran', (tester) async {
      final h = _Harness(makeRideService(DioAdapterMock(data: {
        'success': true,
        'data': {
          'order_id': 'r1',
          'driver_id': '00000000-0000-0000-0000-0000000000d1',
          'status': 'COMPLETED',
          'pickup_lat': -6.2088,
          'pickup_lng': 106.8456,
          'dropoff_lat': -6.2950,
          'dropoff_lng': 106.8638,
          'pickup_address': 'Jl. Sudirman 99',
          'dropoff_address': 'Jl. Gatot Subroto',
          'distance_km': '13.2',
          'estimated_fare': '25000',
          'actual_fare': '24000',
          'discount_amount': '2000',
          'payment_method': 'WALLET',
          'created_at': '2026-09-22T10:00:00Z',
          'completed_at': '2026-09-22T10:30:00Z',
          'settled_at': '2026-09-22T10:31:00Z',
        },
      })));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.rideDetail, args: const RideDetailArgs(orderId: 'r1'));

      expect(find.text('Detail Perjalanan'), findsOneWidget);
      expect(find.text('COMPLETED'), findsOneWidget);
      expect(find.text('Driver #00000000'), findsOneWidget);
      expect(find.text('Rp 24.000'), findsOneWidget);
      expect(find.text('WALLET'), findsWidgets);
      expect(find.widgetWithText(Chip, 'WALLET'), findsOneWidget);
      expect(find.text('Perjalanan selesai'), findsOneWidget);
    });

    testWidgets('error state menampilkan tombol Coba lagi', (tester) async {
      final h = _Harness(makeRideService(DioAdapterMock(data: const {}, statusCode: 500)));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.rideDetail, args: const RideDetailArgs(orderId: 'r1'));

      expect(find.text('Coba lagi'), findsOneWidget);
      expect(
        find.text('Gagal memuat detail perjalanan (jaringan/API). Pastikan server aktif.'),
        findsOneWidget,
      );
    });
  });
}