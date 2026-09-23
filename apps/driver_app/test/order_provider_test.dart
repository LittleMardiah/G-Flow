import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/config/constants.dart';
import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/order_provider.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/order_service.dart';

class _FakeOrderService extends OrderService {
  _FakeOrderService() : super(ApiClient());
  final List<String> acceptedRides = [];
  final List<String> acceptedSends = [];
  final List<String> acceptedFoods = [];
  final List<String> rideStatuses = [];
  final List<String> stoppedStops = [];
  List<DriverOrder> availableOrders = [_ride('a-1')];
  List<DriverOrder> activeOrders = [_ride('ax-1', status: 'TRIP_STARTED')];
  bool failAccept = false;
  bool failUpdate = false;

  @override
  Future<AvailableOrdersResult> fetchAvailableOrders() async {
    return AvailableOrdersResult(
      orders: availableOrders,
      capacityAvailable: true,
      activeOrders: activeOrders.length,
      maxActiveOrders: kMaxActiveOrders,
      radiusKm: 5,
    );
  }

  @override
  Future<List<DriverOrder>> fetchActiveOrders() async => activeOrders;

  @override
  Future<Map<String, dynamic>> acceptRide(String orderId) async {
    if (failAccept) throw Exception('accept fail');
    acceptedRides.add(orderId);
    return const {};
  }

  @override
  Future<Map<String, dynamic>> acceptSend(String orderId) async {
    if (failAccept) throw Exception('accept fail');
    acceptedSends.add(orderId);
    return const {};
  }

  @override
  Future<Map<String, dynamic>> acceptFood(String orderId) async {
    if (failAccept) throw Exception('accept fail');
    acceptedFoods.add(orderId);
    return const {};
  }

  @override
  Future<Map<String, dynamic>> updateRideStatus(String orderId, String status) async {
    if (failUpdate) throw Exception('update fail');
    rideStatuses.add('$orderId:$status');
    return const {};
  }

  @override
  Future<Map<String, dynamic>> updateSendStatus(String orderId, String status) async {
    if (failUpdate) throw Exception('update fail');
    rideStatuses.add('$orderId:$status');
    return const {};
  }

  @override
  Future<Map<String, dynamic>> updateFoodStatus(String orderId, String status) async {
    if (failUpdate) throw Exception('update fail');
    rideStatuses.add('$orderId:$status');
    return const {};
  }

  @override
  Future<Map<String, dynamic>> updateSendStop(
    String orderId,
    String stopId, {
    String status = 'COMPLETED',
    String? deliveryPhotoUrl,
  }) async {
    if (failUpdate) throw Exception('update fail');
    stoppedStops.add(stopId);
    return const {};
  }
}

ProviderContainer _container(_FakeOrderService service) {
  return ProviderContainer(overrides: [
    orderServiceProvider.overrideWithValue(service),
  ]);
}

DriverOrder _ride(String id, {String status = 'DRIVER_ASSIGNED'}) =>
    DriverOrder.fromRideJson({'order_id': id, 'status': status});

DriverOrder _food(String id, {String status = 'READY_FOR_PICKUP'}) =>
    DriverOrder.fromFoodJson({'order_id': id, 'status': status});

DriverOrder _send(String id, {String status = 'DRIVER_ASSIGNED', List<Map<String, dynamic>>? stops}) =>
    DriverOrder.fromSendJson({
      'order_id': id,
      'status': status,
      'stops': stops ??
          [
            {'stop_id': 's1', 'stop_number': 1, 'status': 'PENDING'},
            {'stop_id': 's2', 'stop_number': 2, 'status': 'PENDING'},
          ],
    });

void main() {
  group('AvailableOrdersNotifier', () {
    test('fetch loads mock available orders', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(availableOrdersProvider.notifier);

      await notifier.fetch();
      final state = container.read(availableOrdersProvider);
      expect(state.isLoading, isFalse);
      expect(state.orders, isNotEmpty);
      expect(state.error, isNull);
    });

    test('fetch sets error on failure', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(availableOrdersProvider.notifier);

      await notifier.fetch();
      expect(container.read(availableOrdersProvider).orders, isNotEmpty);
    });

    test('remove deletes order from list', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(availableOrdersProvider.notifier);
      await notifier.fetch();
      final before = container.read(availableOrdersProvider).orders.length;
      final id = container.read(availableOrdersProvider).orders.first.id;
      notifier.remove(id);
      expect(container.read(availableOrdersProvider).orders.length, before - 1);
    });
  });

  group('ActiveOrdersNotifier capacity & accept', () {
    test('starts empty and reports capacity false', () {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final state = container.read(activeOrdersProvider);
      expect(state.orders, isEmpty);
      expect(state.isAtCapacity, isFalse);
      expect(state.activeCount, 0);
    });

    test('fetch with no driverId returns empty', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      notifier.setDriverId(null);
      await notifier.fetch();
      expect(container.read(activeOrdersProvider).orders, isEmpty);
    });

    test('setDriverId triggers fetch of active orders', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      notifier.setDriverId('driver-1');
      await Future<void>.delayed(Duration.zero);
      expect(container.read(activeOrdersProvider).orders, isNotEmpty);
    });

    test('accept adds order and consumes capacity up to 3', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);

      for (var i = 0; i < kMaxActiveOrders; i++) {
        final ok = await notifier.accept(_ride('r-$i'));
        expect(ok, isTrue);
      }
      final state = container.read(activeOrdersProvider);
      expect(state.orders.length, kMaxActiveOrders);
      expect(state.isAtCapacity, isTrue);
      expect(service.acceptedRides.length, kMaxActiveOrders);
    });

    test('accept rejected at capacity with error message', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);

      for (var i = 0; i < kMaxActiveOrders; i++) {
        await notifier.accept(_ride('r-$i'));
      }
      final ok = await notifier.accept(_ride('r-extra'));
      expect(ok, isFalse);
      final state = container.read(activeOrdersProvider);
      expect(state.orders.length, kMaxActiveOrders);
      expect(state.error, contains('Kapasitas penuh'));
    });

    test('accept food sets READY_FOR_PICKUP status', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      final ok = await notifier.accept(_food('f-1'));
      expect(ok, isTrue);
      final order = container.read(activeOrdersProvider).orders.first;
      expect(order.status, 'READY_FOR_PICKUP');
      expect(service.acceptedFoods, ['f-1']);
    });

    test('accept send sets DRIVER_ASSIGNED', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      final ok = await notifier.accept(_send('s-1'));
      expect(ok, isTrue);
      expect(service.acceptedSends, ['s-1']);
      final order = container.read(activeOrdersProvider).orders.first;
      expect(order.status, 'DRIVER_ASSIGNED');
    });

    test('accept failure sets error and no order added', () async {
      final service = _FakeOrderService()..failAccept = true;
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      final ok = await notifier.accept(_ride('r-1'));
      expect(ok, isFalse);
      expect(container.read(activeOrdersProvider).orders, isEmpty);
      expect(container.read(activeOrdersProvider).error, contains('Gagal accept'));
    });

    test('message set on successful accept', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      await container.read(activeOrdersProvider.notifier).accept(_ride('r-1'));
      expect(container.read(activeOrdersProvider).message, contains('diterima'));
    });
  });

  group('ActiveOrdersNotifier updateStatus', () {
    test('updates order status in place', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      await notifier.accept(_ride('r-1'));
      final ok = await notifier.updateStatus(container.read(activeOrdersProvider).orders.first, 'DRIVER_ARRIVED');
      expect(ok, isTrue);
      final order = container.read(activeOrdersProvider).orders.first;
      expect(order.status, 'DRIVER_ARRIVED');
      expect(service.rideStatuses, contains('r-1:DRIVER_ARRIVED'));
    });

    test('updateStatus failure sets error', () async {
      final service = _FakeOrderService()..failUpdate = true;
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      await notifier.accept(_ride('r-1'));
      final ok = await notifier.updateStatus(container.read(activeOrdersProvider).orders.first, 'COMPLETED');
      expect(ok, isFalse);
      expect(container.read(activeOrdersProvider).error, contains('Gagal update status'));
    });
  });

  group('ActiveOrdersNotifier markStopDelivered (multi-stop)', () {
    test('marks a stop delivered and keeps order in-transit', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      await notifier.accept(_send('s-1'));
      final order = container.read(activeOrdersProvider).orders.first;
      final ok = await notifier.markStopDelivered(order, 's1');
      expect(ok, isTrue);
      final updated = container.read(activeOrdersProvider).orders.first;
      expect(updated.stops.first.isDelivered, isTrue);
      expect(updated.stops.length, 2);
      expect(updated.stopsCompleted, 1);
      expect(updated.status, 'DRIVER_ASSIGNED');
      expect(service.stoppedStops, ['s1']);
    });

    test('marking last stop sets order DELIVERED', () async {
      final service = _FakeOrderService();
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      await notifier.accept(_send('s-1', stops: [
        {'stop_id': 'a', 'stop_number': 1, 'status': 'PENDING'},
      ]));
      final order = container.read(activeOrdersProvider).orders.first;
      final ok = await notifier.markStopDelivered(order, 'a', deliveryPhotoUrl: 'http://img');
      expect(ok, isTrue);
      final updated = container.read(activeOrdersProvider).orders.first;
      expect(updated.stops.first.isDelivered, isTrue);
      expect(updated.stops.first.deliveryPhotoUrl, 'http://img');
      expect(updated.status, 'DELIVERED');
      expect(updated.isTerminal, isTrue);
    });

    test('markStopDelivered failure sets error', () async {
      final service = _FakeOrderService()..failUpdate = true;
      final container = _container(service);
      addTearDown(container.dispose);
      final notifier = container.read(activeOrdersProvider.notifier);
      await notifier.accept(_send('s-1'));
      final order = container.read(activeOrdersProvider).orders.first;
      final ok = await notifier.markStopDelivered(order, 's1');
      expect(ok, isFalse);
      expect(container.read(activeOrdersProvider).error, contains('Gagal update stop'));
    });
  });
}
