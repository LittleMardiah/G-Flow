import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/models/driver_earning.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/earnings_service.dart';
import 'package:driver_app/services/order_service.dart';

void main() {
  group('OrderService mock paths', () {
    final service = OrderService(ApiClient(dio: Dio(BaseOptions())));

    test('fetchAvailableRides returns mock rides', () async {
      final rides = await service.fetchAvailableRides();
      expect(rides, isNotEmpty);
      expect(rides.every((o) => o.type == OrderType.ride), isTrue);
      expect(rides.every((o) => o.isMock), isTrue);
    });

    test('fetchAvailableFoodOrders returns mock foods', () async {
      final foods = await service.fetchAvailableFoodOrders();
      expect(foods, isNotEmpty);
      expect(foods.every((o) => o.type == OrderType.food), isTrue);
    });

    test('fetchAvailableSendOrders returns mock sends with stops', () async {
      final sends = await service.fetchAvailableSendOrders();
      expect(sends, isNotEmpty);
      expect(sends.first.type, OrderType.send);
      expect(sends.first.hasStops, isTrue);
      expect(sends.first.stops.length, 3);
    });

    test('fetchAvailableOrders combines and sorts by distance', () async {
      final orders = await service.fetchAvailableOrders();
      expect(orders.length,
          (await service.fetchAvailableRides()).length +
              (await service.fetchAvailableFoodOrders()).length +
              (await service.fetchAvailableSendOrders()).length);
      for (var i = 1; i < orders.length; i++) {
        expect(orders[i].distanceKm >= orders[i - 1].distanceKm, isTrue);
      }
    });

    test('fetchActiveOrders returns mock active orders for driver', () async {
      final orders = await service.fetchActiveOrders('driver-1');
      expect(orders, isNotEmpty);
      expect(orders.any((o) => o.type == OrderType.send), isTrue);
      expect(orders.any((o) => o.type == OrderType.ride), isTrue);
    });

    test('acceptFood returns true after delay', () async {
      expect(await service.acceptFood('food-1'), isTrue);
    });
  });

  group('OrderService.fetchRideDetail', () {
    test('falls back to mock order on DioException', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://unreachable.invalid'));
      intercept404(dio);
      final service = OrderService(ApiClient(dio: dio));
      final order = await service.fetchRideDetail('mock-ride-2351');
      expect(order.id, 'mock-ride-2351');
    });

    test('returns fallback order for unknown id', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://unreachable.invalid'));
      intercept404(dio);
      final service = OrderService(ApiClient(dio: dio));
      final order = await service.fetchRideDetail('unknown-id');
      expect(order.id, 'unknown-id');
      expect(order.status, 'DRIVER_ASSIGNED');
      expect(order.isMock, isTrue);
    });
  });

  group('EarningsService mock path', () {
    test('fetchEarnings returns mock earning', () async {
      final service = EarningsService(ApiClient(dio: Dio(BaseOptions())));
      final e = await service.fetchEarnings('driver-1');
      expect(e, isA<DriverEarning>());
      expect(e.isMock, isTrue);
      expect(e.daily, isNotEmpty);
      expect(e.weekly.length, 7);
      expect(e.todayTotal, 285000);
      expect(e.todayOrderCount, 12);
      expect(e.weekTotal, 1675000);
      expect(e.rideCount, 6);
      expect(e.foodCount, 5);
      expect(e.sendCount, 1);
      expect(e.totalOrderCount, 12);
    });
  });
}

void intercept404(Dio dio) {
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) =>
          handler.reject(DioException(requestOptions: options, type: DioExceptionType.connectionError)),
    ),
  );
}
