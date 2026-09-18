import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/models/driver_earning.dart';
import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/models/driver_stop.dart';

void main() {
  group('OrderType', () {
    test('fromKey maps known keys and defaults to ride', () {
      expect(OrderType.fromKey('ride'), OrderType.ride);
      expect(OrderType.fromKey('FOOD'), OrderType.food);
      expect(OrderType.fromKey('Send'), OrderType.send);
      expect(OrderType.fromKey('unknown'), OrderType.ride);
      expect(OrderType.fromKey(null), OrderType.ride);
      expect(OrderType.fromKey(123), OrderType.ride);
    });

    test('labels are exposed', () {
      expect(OrderType.ride.label, 'Ride');
      expect(OrderType.food.label, 'Food');
      expect(OrderType.send.label, 'Send');
      expect(OrderType.ride.key, 'ride');
    });
  });

  group('DriverOrder active/terminal status sets', () {
    test('ride active statuses', () {
      for (final s in ['DRIVER_ASSIGNED', 'DRIVER_ARRIVED', 'TRIP_STARTED']) {
        final o = DriverOrder.fromRideJson({'order_id': 'x', 'status': s});
        expect(o.isActive, isTrue, reason: s);
      }
      final inactive = DriverOrder.fromRideJson({'order_id': 'x', 'status': 'SEARCHING_DRIVER'});
      expect(inactive.isActive, isFalse);
    });

    test('food active statuses', () {
      for (final s in ['READY_FOR_PICKUP', 'PICKED_UP', 'IN_TRANSIT']) {
        final o = DriverOrder.fromFoodJson({'order_id': 'x', 'status': s});
        expect(o.isActive, isTrue, reason: s);
      }
      final inactive = DriverOrder.fromFoodJson({'order_id': 'x', 'status': 'CONFIRMED'});
      expect(inactive.isActive, isFalse);
    });

    test('send active statuses', () {
      for (final s in ['DRIVER_ASSIGNED', 'PICKED_UP', 'IN_TRANSIT']) {
        final o = DriverOrder.fromSendJson({'order_id': 'x', 'status': s});
        expect(o.isActive, isTrue, reason: s);
      }
      final inactive = DriverOrder.fromSendJson({'order_id': 'x', 'status': 'SEARCHING_DRIVER'});
      expect(inactive.isActive, isFalse);
    });

    test('terminal statuses', () {
      for (final s in ['COMPLETED', 'SETTLED', 'CANCELLED', 'DELIVERED']) {
        final o = DriverOrder.fromRideJson({'order_id': 'x', 'status': s});
        expect(o.isTerminal, isTrue, reason: s);
      }
    });
  });

  group('DriverOrder.fromRideJson edge cases', () {
    test('defaults when fields missing', () {
      final o = DriverOrder.fromRideJson({});
      expect(o.id, '');
      expect(o.status, 'SEARCHING_DRIVER');
      expect(o.pickupLat, 0);
      expect(o.distanceKm, 0);
      expect(o.estimatedFare, 0);
      expect(o.paymentMethod, 'WALLET');
      expect(o.createdAt, isNull);
      expect(o.isMock, isFalse);
    });

    test('parses id from top-level id', () {
      final o = DriverOrder.fromRideJson({'id': 'r-9'});
      expect(o.id, 'r-9');
    });

    test('derives driverEarning from 80% of estimated_fare when not given', () {
      final o = DriverOrder.fromRideJson({'estimated_fare': 100000});
      expect(o.driverEarning, 80000);
    });

    test('parses createdAt date string', () {
      final o = DriverOrder.fromRideJson({'created_at': '2026-01-01T10:00:00Z'});
      expect(o.createdAt, isNotNull);
    });
  });

  group('DriverOrder.fromFoodJson edge cases', () {
    test('merchant nested map parsing', () {
      final o = DriverOrder.fromFoodJson({
        'order_id': 'f-1',
        'merchant': {'name': 'Warung', 'address': 'Jl. A', 'location_lat': -6.2, 'location_lng': 106.8},
        'delivery_address': 'Jl. B',
        'delivery_lat': -6.3,
        'delivery_lng': 106.9,
      });
      expect(o.merchantName, 'Warung');
      expect(o.merchantAddress, 'Jl. A');
      expect(o.pickupLat, -6.2);
      expect(o.pickupLng, 106.8);
      expect(o.deliveryAddress, 'Jl. B');
    });

    test('driverEarning falls back to delivery_fee', () {
      final o = DriverOrder.fromFoodJson({'delivery_fee': 8000});
      expect(o.driverEarning, 8000);
    });

    test('defaults', () {
      final o = DriverOrder.fromFoodJson({});
      expect(o.type, OrderType.food);
      expect(o.status, 'CONFIRMED');
      expect(o.merchantName, '');
    });
  });

  group('DriverOrder.fromSendJson', () {
    test('delivery info comes from first stop', () {
      final o = DriverOrder.fromSendJson({
        'order_id': 's-1',
        'stops': [
          {'stop_id': 'a', 'address': 'Jl. X', 'lat': -6.1, 'lng': 106.7},
        ],
      });
      expect(o.deliveryAddress, 'Jl. X');
      expect(o.deliveryLat, -6.1);
      expect(o.deliveryLng, 106.7);
      expect(o.hasStops, isTrue);
    });

    test('uses total_distance_km and total_fare', () {
      final o = DriverOrder.fromSendJson({
        'order_id': 's-1',
        'total_distance_km': 6.3,
        'total_fare': 45000,
        'driver_earning': 36000,
        'stops': [],
      });
      expect(o.distanceKm, 6.3);
      expect(o.estimatedFare, 45000);
      expect(o.driverEarning, 36000);
      expect(o.deliveryAddress, '');
    });
  });

  group('DriverOrder copyWith', () {
    test('updates status and stops', () {
      final o = DriverOrder.fromSendJson({
        'order_id': 's-1',
        'stops': [
          {'stop_id': 'a', 'status': 'PENDING'},
        ],
      });
      final copy = o.copyWith(status: 'DELIVERED');
      expect(copy.status, 'DELIVERED');
      expect(copy.id, o.id);
      expect(identical(copy.stops, o.stops), isTrue);
    });
  });

  group('DriverOrder nextStop helpers', () {
    test('nextUndeliveredStop returns first non-delivered', () {
      final o = DriverOrder.fromSendJson({
        'order_id': 's-1',
        'stops': [
          {'stop_id': 'a', 'status': 'COMPLETED'},
          {'stop_id': 'b', 'status': 'PENDING'},
          {'stop_id': 'c', 'status': 'PENDING'},
        ],
      });
      expect(o.nextUndeliveredStop?.id, 'b');
      expect(o.stopsCompleted, 1);
    });

    test('nextUndeliveredStop null when all delivered', () {
      final o = DriverOrder.fromSendJson({
        'order_id': 's-1',
        'stops': [
          {'stop_id': 'a', 'status': 'COMPLETED'},
        ],
      });
      expect(o.nextUndeliveredStop, isNull);
      expect(o.stopsCompleted, 1);
    });

    test('nextUndeliveredStop null when no stops', () {
      final o = DriverOrder.fromSendJson({'order_id': 's-1', 'stops': []});
      expect(o.nextUndeliveredStop, isNull);
      expect(o.hasStops, isFalse);
    });
  });

  group('DriverStop', () {
    test('isDelivered is case insensitive', () {
      final o = DriverStop.fromJson({'status': 'completed'});
      expect(o.isDelivered, isTrue);
    });

    test('parses alternate field names', () {
      final s = DriverStop.fromJson({
        'id': 'd',
        'order_id': 'ord',
        'order': 3,
        'name': 'Ria',
        'phone': '081',
        'dropoff_address': 'Jl. Y',
        'dropoff_lat': -6.5,
        'dropoff_lng': 106.5,
        'delivery_photo_url': 'http://img',
      });
      expect(s.id, 'd');
      expect(s.orderId, 'ord');
      expect(s.stopNumber, 3);
      expect(s.recipientName, 'Ria');
      expect(s.recipientPhone, '081');
      expect(s.address, 'Jl. Y');
      expect(s.latitude, -6.5);
      expect(s.longitude, 106.5);
      expect(s.deliveryPhotoUrl, 'http://img');
    });

    test('parses location_lat/location_lng', () {
      final s = DriverStop.fromJson({
        'stop_id': 'd',
        'location_lat': 1.1,
        'location_lng': 2.2,
        'latitude': 3.3,
        'longitude': 4.4,
      });
      expect(s.latitude, 1.1);
      expect(s.longitude, 2.2);
    });

    test('defaults fields', () {
      final s = DriverStop.fromJson({});
      expect(s.id, '');
      expect(s.stopNumber, 0);
      expect(s.status, 'PENDING');
      expect(s.isDelivered, isFalse);
    });
  });

  group('DriverEarning', () {
    test('totalOrderCount sums counts', () {
      const e = DriverEarning(rideCount: 1, foodCount: 2, sendCount: 3);
      expect(e.totalOrderCount, 6);
    });

    test('parses weekly points', () {
      final e = DriverEarning.fromJson({
        'week_total': 100,
        'weekly': [
          {'label': 'Sen', 'amount': 100, 'order_count': 5},
        ],
      });
      expect(e.weekTotal, 100);
      expect(e.weekly.length, 1);
      expect(e.weekly.first.label, 'Sen');
      expect(e.weekly.first.orderCount, 5);
    });

    test('requires label from date field', () {
      final p = EarningPoint.fromJson({'date': '2026-01-01', 'total': 50, 'orders': 2});
      expect(p.label, '2026-01-01');
      expect(p.amount, 50);
      expect(p.orderCount, 2);
    });

    test('defaults missing earning fields to zero', () {
      final e = DriverEarning.fromJson({});
      expect(e.todayTotal, 0);
      expect(e.todayOrderCount, 0);
      expect(e.rideCount, 0);
      expect(e.foodCount, 0);
      expect(e.sendCount, 0);
      expect(e.daily, isEmpty);
      expect(e.isMock, isFalse);
    });

    test('parses alternate field names', () {
      final e = DriverEarning.fromJson({'total_earnings': 500, 'order_count': 3});
      expect(e.todayTotal, 500);
      expect(e.todayOrderCount, 3);
    });
  });
}
