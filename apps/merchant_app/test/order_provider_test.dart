import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:merchant_app/models/merchant_order.dart';
import 'package:merchant_app/providers/auth_provider.dart';
import 'package:merchant_app/providers/order_provider.dart';
import 'package:merchant_app/services/api_client.dart';
import 'package:merchant_app/services/order_service.dart';

class FakeDioAdapter implements HttpClientAdapter {
  final List<Map<String, dynamic>> responses;
  final List<Object> errors;
  int callCount = 0;
  final List<String> paths = [];

  FakeDioAdapter({this.responses = const [], this.errors = const []});

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    final idx = callCount++;
    paths.add(options.uri.toString());
    if (idx < errors.length) {
      final e = errors[idx];
      if (e is DioException) throw e;
      throw e;
    }
    final body = idx < responses.length ? _encode(responses[idx]['body'] ?? {}) : '{"data":{}}';
    return ResponseBody.fromString(body, responses[idx]['status'] ?? 200, headers: const {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
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
  final Map<String, String> store = {};
  @override
  Future<String?> read({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key];
  @override
  Future<void> write({required String key, required String? value, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key] = value ?? '';
  @override
  Future<void> delete({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store.remove(key);
}

ApiClient _api(FakeDioAdapter adapter, FakeStorage storage) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test'));
  dio.httpClientAdapter = adapter;
  return ApiClient(dio: dio, storage: storage);
}

Map<String, dynamic> _orderJson(String id, String status, String merchantStatus, {int total = 1000}) => {
  'id': id,
  'status': status,
  'merchant_status': merchantStatus,
  'total_amount': total,
  'items': [
    {'item_name': 'X', 'quantity': 1, 'subtotal': total},
  ],
};

void main() {
  late FakeStorage storage;
  late ProviderContainer container;

  tearDown(() {
    container.dispose();
  });

  group('OrdersNotifier', () {
    test('load populates orders and visibleOrders for ALL', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      container = ProviderContainer(overrides: [storageProvider.overrideWithValue(storage)]);
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [_orderJson('o1', 'CREATED', 'WAITING')]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);

      await notifier.load();

      expect(notifier.state.orders.length, 1);
      expect(notifier.state.filter, null);
      expect(notifier.visibleOrders.length, 1);
    });

    test('visibleOrders filters by displayStatus when filter set', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [
          _orderJson('o1', 'CREATED', 'WAITING'),
          _orderJson('o2', 'PREPARING', 'PREPARING'),
        ]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load(status: 'WAITING');

      final filtered = notifier.state.orders.where((o) => o.displayStatus == 'WAITING').toList();
      expect(filtered.length, 1);
    });

    test('load sends status=CANCELLED so backend filters by order status (TD-140)', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [_orderJson('o1', 'CANCELLED', 'WAITING')]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load(status: 'CANCELLED');

      expect(adapter.paths.single, contains('status=CANCELLED'));
      expect(notifier.state.visibleOrders.length, 1);
    });

    test('WAITING tab hides rejected order (status CANCELLED, merchant_status WAITING) (TD-140)', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      // Backend OR-filter mengembalikan order reject ini di tab WAITING karena
      // merchant_status='WAITING' (repository.go:912).
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [
          _orderJson('o-rejected', 'CANCELLED', 'WAITING'),
          _orderJson('o-live', 'CREATED', 'WAITING'),
        ]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load(status: 'WAITING');

      expect(adapter.paths.single, contains('status=WAITING'));
      expect(notifier.state.orders.length, 2, reason: 'backend tetap mengirim 2 order');
      expect(notifier.state.visibleOrders.length, 1, reason: 'order reject disaring client-side');
      expect(notifier.state.visibleOrders.single.id, 'o-live');
      expect(notifier.state.visibleOrders.single.displayStatus, 'WAITING');
    });

    test('CANCELLED tab shows rejected order and hides live WAITING order (TD-140)', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [
          _orderJson('o-rejected', 'CANCELLED', 'WAITING'),
          _orderJson('o-live', 'CREATED', 'WAITING'),
        ]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load(status: 'CANCELLED');

      expect(notifier.state.visibleOrders.length, 1);
      expect(notifier.state.visibleOrders.single.id, 'o-rejected');
      expect(notifier.state.visibleOrders.single.displayStatus, 'CANCELLED');
    });

    test('ALL tab keeps both rejected and live orders visible (TD-140)', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [
          _orderJson('o-rejected', 'CANCELLED', 'WAITING'),
          _orderJson('o-live', 'CREATED', 'WAITING'),
        ]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load(status: 'ALL');

      expect(adapter.paths.single, isNot(contains('status=')));
      expect(notifier.state.visibleOrders.length, 2);
    });

    test('OrdersState.visibleOrders pure logic without notifier', () {
      const rejected = MerchantOrder(id: 'a', status: 'CANCELLED', merchantStatus: 'WAITING');
      const live = MerchantOrder(id: 'b', status: 'CREATED', merchantStatus: 'WAITING');
      const s = OrdersState(orders: [rejected, live], filter: 'WAITING');
      expect(s.visibleOrders.map((o) => o.id), ['b']);
      expect(const OrdersState(orders: [rejected, live]).visibleOrders.length, 2);
      expect(const OrdersState(orders: [rejected, live], filter: 'ALL').visibleOrders.length, 2);
    });

    test('load sets error when merchant id missing', () async {
      storage = FakeStorage();
      container = ProviderContainer(overrides: [storageProvider.overrideWithValue(storage)]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load();
      expect(notifier.state.error, isNotNull);
    });

    test('load sets error on failure', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(errors: [DioException(requestOptions: RequestOptions(path: '/x'), message: 'boom')]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load();
      expect(notifier.state.error, isNotNull);
      expect(notifier.state.isLoading, isFalse);
    });

    test('setStatus transitions and reloads', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      // load() with no filter then setStatus which reloads with filter
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [_orderJson('o1', 'CREATED', 'WAITING')]}}},
        {'body': {'data': {'ok': true}}},
        {'body': {'data': {'orders': [_orderJson('o1', 'CONFIRMED', 'CONFIRMED')]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load();

      final ok = await notifier.setStatus('o1', 'CONFIRMED');
      expect(ok, isTrue);
    });

    test('setStatus returns false on failure', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(errors: [DioException(requestOptions: RequestOptions(path: '/x'), message: 'boom')]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      final ok = await notifier.setStatus('o1', 'CONFIRMED');
      expect(ok, isFalse);
      expect(notifier.state.error, isNotNull);
    });

    test('orderById finds order', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [_orderJson('o1', 'CREATED', 'WAITING')]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final notifier = container.read(orderProvider.notifier);
      await notifier.load();
      expect(notifier.orderById('o1')?.id, 'o1');
      expect(notifier.orderById('nope'), isNull);
    });

    test('OrdersState.copyWith', () {
      const s = OrdersState();
      final c = s.copyWith(orders: [], isLoading: true, error: 'e', filter: 'ALL');
      expect(c.isLoading, isTrue);
      expect(c.filter, 'ALL');
    });

    test('orderDetailProvider fetches detail', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'order': {'id': 'o1', 'status': 'PREPARING'}, 'items': [{'item_name': 'Nasi', 'quantity': 2}]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      final f = container.read(orderDetailProvider('o1').future);
      final o = await f;
      expect(o.id, 'o1');
    });
  });
}
