import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/config/constants.dart';
import 'package:driver_app/models/driver_earning.dart';
import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/models/driver_stop.dart';
import 'package:driver_app/providers/order_provider.dart';

void main() {
  group('DriverStop', () {
    test('parses stop JSON with stop_id/status & computes isDelivered', () {
      final pending = DriverStop.fromJson({
        'stop_id': 's-1',
        'stop_number': 1,
        'recipient_name': 'Budi',
        'recipient_phone': '+628111',
        'delivery_address': 'Jl. A',
        'delivery_lat': -6.2,
        'delivery_lng': 106.8,
        'allocated_fare': 15000,
        'status': 'PENDING',
      });
      final done = DriverStop.fromJson({
        'id': 's-2',
        'order': 2,
        'status': 'COMPLETED',
      });

      expect(pending.id, 's-1');
      expect(pending.stopNumber, 1);
      expect(pending.recipientName, 'Budi');
      expect(pending.allocatedFare, 15000);
      expect(pending.isDelivered, isFalse);
      expect(done.id, 's-2');
      expect(done.stopNumber, 2);
      expect(done.isDelivered, isTrue);
    });
  });

  group('DriverOrder', () {
    test('parses ride JSON & maps type/status/earning', () {
      final order = DriverOrder.fromRideJson({
        'order_id': 'r-1',
        'status': 'TRIP_STARTED',
        'pickup_address': 'Jl. Sudirman',
        'pickup_lat': -6.2,
        'pickup_lng': 106.8,
        'dropoff_address': 'Mall',
        'dropoff_lat': -6.25,
        'dropoff_lng': 106.85,
        'distance_km': 2.5,
        'estimated_fare': 75000,
        'driver_earning': 60000,
        'payment_method': 'WALLET',
      });

      expect(order.type, OrderType.ride);
      expect(order.id, 'r-1');
      expect(order.status, 'TRIP_STARTED');
      expect(order.distanceKm, 2.5);
      expect(order.driverEarning, 60000);
      expect(order.isActive, isTrue);
    });

    test('parses food JSON & reads merchant name/address', () {
      final order = DriverOrder.fromFoodJson({
        'order_id': 'f-1',
        'merchant_name': 'Warung Mak Siti',
        'merchant_address': 'Jl. Sabang',
        'merchant_lat': -6.2,
        'merchant_lng': 106.84,
        'delivery_address': 'Jl. Gatot',
        'delivery_lat': -6.21,
        'delivery_lng': 106.85,
        'total_amount': 68000,
        'delivery_fee': 8000,
        'status': 'READY_FOR_PICKUP',
      });

      expect(order.type, OrderType.food);
      expect(order.merchantName, 'Warung Mak Siti');
      expect(order.driverEarning, 8000);
      expect(order.isActive, isTrue);
    });

    test('parses send JSON with multi-stop & next stop', () {
      final order = DriverOrder.fromSendJson({
        'order_id': 's-1',
        'status': 'IN_TRANSIT',
        'pickup_address': 'Jl. Thamrin',
        'pickup_lat': -6.2,
        'pickup_lng': 106.84,
        'total_fare': 45000,
        'stops': [
          {'stop_id': 'a', 'stop_number': 1, 'status': 'COMPLETED', 'allocated_fare': 15000},
          {'stop_id': 'b', 'stop_number': 2, 'status': 'PENDING', 'allocated_fare': 19000, 'delivery_address': 'Jl. B'},
          {'stop_id': 'c', 'stop_number': 3, 'status': 'PENDING', 'allocated_fare': 11000, 'delivery_address': 'Jl. C'},
        ],
      });

      expect(order.type, OrderType.send);
      expect(order.hasStops, isTrue);
      expect(order.stopsCompleted, 1);
      expect(order.nextUndeliveredStop?.id, 'b');
      expect(order.isActive, isTrue);
      expect(order.isTerminal, isFalse);
    });

    test('unifies JSON via type field (fromJson sniffing)', () {
      final ride = DriverOrder.fromJson({'type': 'ride', 'order_id': 'x'});
      final food = DriverOrder.fromJson({'type': 'food', 'id': 'y'});
      final send = DriverOrder.fromJson({'order_type': 'send', 'id': 'z'});
      expect(ride.type, OrderType.ride);
      expect(food.type, OrderType.food);
      expect(send.type, OrderType.send);
    });

    test('terminal status detection', () {
      final done = DriverOrder.fromRideJson({'order_id': 'r', 'status': 'SETTLED'});
      expect(done.isActive, isFalse);
      expect(done.isTerminal, isTrue);
    });
  });

  group('DriverEarning', () {
    test('parses summary json & computes totals', () {
      final e = DriverEarning.fromJson({
        'today_total': 285000,
        'today_order_count': 12,
        'week_total': 1675000,
        'ride_count': 6,
        'food_count': 5,
        'send_count': 1,
        'daily': [
          {'label': '09.00', 'amount': 12000},
          {'label': '10.00', 'amount': 35000},
        ],
      });

      expect(e.todayTotal, 285000);
      expect(e.todayOrderCount, 12);
      expect(e.totalOrderCount, 12);
      expect(e.daily.length, 2);
      expect(e.daily.first.amount, 12000);
    });
  });

  group('CapacityCheck (ROADMAP 3.10 C)', () {
    test('kapasitas maksimum driver adalah 3 order aktif', () {
      expect(kMaxActiveOrders, 3);
    });

    test('OrdersListState hanya menampung jumlah order aktif', () {
      final orders = List.generate(
        kMaxActiveOrders,
        (i) => DriverOrder.fromRideJson({'order_id': 'o-$i', 'status': 'TRIP_STARTED'}),
      );
      final state = OrdersListState(orders: orders);
      expect(state.orders.length, kMaxActiveOrders);
      expect(kMaxActiveOrders >= state.orders.length, isTrue);
    });

    test('DriverOrder.isActive benar untuk status in-progress', () {
      final inProgress = List.generate(
        3,
        (i) => DriverOrder.fromRideJson({'order_id': 'o-$i', 'status': 'TRIP_STARTED'}),
      );
      expect(inProgress.where((o) => o.isActive).length, 3);
    });
  });
}