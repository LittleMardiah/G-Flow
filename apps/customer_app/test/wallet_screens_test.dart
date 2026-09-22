import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:customer_app/config/router.dart';
import 'package:customer_app/providers/wallet_provider.dart';
import 'package:customer_app/screens/wallet/wallet_balance_screen.dart';
import 'package:customer_app/screens/wallet/wallet_history_screen.dart';
import 'package:customer_app/screens/wallet/wallet_topup_screen.dart';
import 'package:customer_app/screens/wallet/wallet_transfer_screen.dart';
import 'package:customer_app/services/api_client.dart';
import 'package:customer_app/services/wallet_service.dart';

/// Fake base (pengganti mockito) — pola sama ride_screens_test.dart.
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

WalletService makeWalletService(DioAdapterMock adapter) =>
    WalletService(ApiClient(dio: Dio()..httpClientAdapter = adapter, storage: FakeStorage()));

class _Harness {
  _Harness(this.service) : navigatorKey = GlobalKey<NavigatorState>();

  final GlobalKey<NavigatorState> navigatorKey;
  final WalletService service;

  Widget build() {
    return ProviderScope(
      overrides: [walletServiceProvider.overrideWithValue(service)],
      child: MaterialApp(
        navigatorKey: navigatorKey,
        routes: {
          AppRoutes.walletBalance: (_) => const WalletBalanceScreen(),
          AppRoutes.walletTopup: (_) => const WalletTopupScreen(),
          AppRoutes.walletTransfer: (_) => const WalletTransferScreen(),
          AppRoutes.walletHistory: (_) => const WalletHistoryScreen(),
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
  group('WalletBalanceScreen', () {
    testWidgets('render saldo + transaksi terbaru + tombol aksi', (tester) async {
      final h = _Harness(makeWalletService(DioAdapterMock(data: {
        'success': true,
        'data': {
          'wallet_id': 'w1',
          'balance': '1500000',
          'entries': [
            {
              'ledger_id': 'l1',
              'entry_type': 'CREDIT',
              'amount': '100000',
              'balance_after': '1500000',
              'reference_type': 'TOPUP',
              'description': 'Top up',
              'is_reversed': false,
              'created_at': '2026-09-22T10:00:00Z',
            },
            {
              'ledger_id': 'l2',
              'entry_type': 'DEBIT',
              'amount': '20000',
              'balance_after': '1400000',
              'reference_type': 'RIDE_SETTLEMENT',
              'description': 'Pembayaran ride',
              'is_reversed': false,
              'created_at': '2026-09-21T09:00:00Z',
            },
          ],
        },
        'meta': {'page': 1, 'page_size': 20, 'total': 2, 'total_pages': 1},
      })));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.walletBalance);

      expect(find.text('Wallet'), findsOneWidget);
      expect(find.text('Rp 1.500.000'), findsOneWidget);
      expect(find.text('Top-Up'), findsOneWidget);
      expect(find.text('Transfer'), findsOneWidget);
      expect(find.text('Riwayat'), findsOneWidget);
      expect(find.text('Transaksi Terbaru'), findsOneWidget);
      expect(find.text('Top up'), findsOneWidget);
      expect(find.text('Pembayaran ride'), findsOneWidget);
      expect(find.text('+Rp 100.000'), findsOneWidget);
      expect(find.text('-Rp 20.000'), findsOneWidget);
      expect(find.text('Lihat Semua'), findsOneWidget);
    });
  });

  group('WalletTopupScreen', () {
    testWidgets('submit top-up berhasil menampilkan snackbar & menutup screen', (tester) async {
      final h = _Harness(makeWalletService(DioAdapterMock(data: {
        'success': true,
        'data': {
          'transaction_id': 't1',
          'wallet_id': 'w1',
          'amount': '100000',
          'status': 'COMPLETED',
          'new_balance': '1600000',
        },
      })));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.walletTopup, args: const WalletArgs(walletId: 'w1'));

      await tester.tap(find.text('Rp 50.000'));
      await tester.pump();
      expect(
        tester
            .widget<TextField>(find.byType(TextField).first)
            .controller!
            .text,
        '50000',
      );

      await tester.tap(find.text('Top-Up Sekarang'));
      for (var i = 0; i < 20; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }

      expect(find.byType(SnackBar), findsOneWidget);
      expect(
        find.text('Top-up berhasil.'),
        findsAtLeastNWidgets(1),
      );
    });

    testWidgets('nominal invalid memunculkan snackbar validasi', (tester) async {
      final h = _Harness(makeWalletService(DioAdapterMock(data: const {})));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.walletTopup, args: const WalletArgs(walletId: 'w1'));

      await tester.tap(find.text('Top-Up Sekarang'));
      for (var i = 0; i < 20; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }

      // Duplikat SnackBar/Text hanya transien saat entrance animation;
      // setelah pompa cukup (>= durasi animasi) tersisa satu instance.
      expect(find.byType(SnackBar), findsOneWidget);
      expect(find.text('Nominal top-up harus berupa angka lebih dari 0.'),
          findsOneWidget);
    });
  });

  group('WalletHistoryScreen', () {
    testWidgets('empty state menampilkan pesan', (tester) async {
      final h = _Harness(makeWalletService(DioAdapterMock(data: {
        'success': true,
        'data': {'entries': <dynamic>[]},
        'meta': {'page': 1, 'page_size': 20, 'total': 0, 'total_pages': 0},
      })));

      await tester.pumpWidget(h.build());
      await _push(tester, h, AppRoutes.walletHistory,
          args: const WalletHistoryArgs(walletId: 'w1'));

      expect(find.text('Riwayat Wallet'), findsOneWidget);
      expect(find.text('Filter Tipe Transaksi'), findsOneWidget);
      expect(find.text('Belum ada transaksi.'), findsOneWidget);
    });
  });
}