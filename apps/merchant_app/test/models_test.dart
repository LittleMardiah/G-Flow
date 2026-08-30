import 'package:flutter_test/flutter_test.dart';

import 'package:merchant_app/models/merchant.dart';
import 'package:merchant_app/models/merchant_item.dart';
import 'package:merchant_app/models/merchant_menu.dart';
import 'package:merchant_app/models/merchant_order.dart';
import 'package:merchant_app/providers/analytics_provider.dart';

void main() {
  group('MerchantOrder', () {
    test('parses backend food order JSON & maps displayStatus', () {
      final order = MerchantOrder.fromJson({
        'id': 'abc-123',
        'customer_id': 'cust-1',
        'merchant_id': 'm-1',
        'status': 'CREATED',
        'merchant_status': 'WAITING',
        'total_amount': 45000,
        'items': [
          {'item_name': 'Nasi Goreng', 'quantity': 2, 'unit_price': 15000, 'subtotal': 30000},
          {'item_name': 'Es Teh', 'quantity': 2, 'subtotal': 15000},
        ],
      });

      expect(order.displayStatus, 'WAITING');
      expect(order.isWaiting, isTrue);
      expect(order.idShort, '#ABC-123');
      expect(order.itemCount, 4);
      expect(order.totalAmount, 45000);
      expect(order.nextActions.length, 2);
      expect(order.nextActions.first.label, 'Confirm');
    });

    test('maps READY_FOR_PICKUP & provides merchant actions', () {
      final order = MerchantOrder.fromJson({
        'id': 'o-2',
        'status': 'PREPARING',
        'merchant_status': 'PREPARING',
      });

      expect(order.isPreparing, isTrue);
      expect(order.nextActions.single.label, 'Ready for Pickup');
      expect(order.nextActions.single.targetStatus, 'READY_FOR_PICKUP');
    });
  });

  group('MerchantItem', () {
    test('parses item JSON with stock & availability', () {
      final item = MerchantItem.fromJson({
        'id': 'i-1',
        'menu_id': 'menu-1',
        'name': 'Ayam Bakar',
        'price': 25000,
        'stock': 3,
        'is_available': false,
        'image_url': 'http://x/img.png',
      });

      expect(item.id, 'i-1');
      expect(item.menuId, 'menu-1');
      expect(item.stock, 3);
      expect(item.isAvailable, isFalse);
      expect(item.outOfStock, isFalse);
    });

    test('default stock is 999', () {
      final item = MerchantItem.fromJson({'id': 'i-2', 'name': 'X', 'price': 1000});
      expect(item.stock, 999);
    });
  });

  group('MerchantMenu', () {
    test('parses menu JSON', () {
      final menu = MerchantMenu.fromJson({
        'id': 'menu-1',
        'merchant_id': 'm-1',
        'name': 'Makanan',
        'sequence_order': 1,
        'is_active': false,
      });

      expect(menu.name, 'Makanan');
      expect(menu.isActive, isFalse);
    });
  });

  group('MerchantProfile', () {
    test('parses GET /merchants/:id response', () {
      final profile = MerchantProfile.fromJson({
        'id': 'm-1',
        'user_id': 'u-1',
        'merchant_name': 'Warung Surti',
        'category': 'makanan',
        'address': 'Jl. Baru',
        'is_open': true,
        'status': 'ACTIVE',
        'avg_rating': 4.5,
        'total_reviews': 12,
        'total_orders': 30,
      });

      expect(profile.merchantName, 'Warung Surti');
      expect(profile.isActive, isTrue);
      expect(profile.isOpen, isTrue);
      expect(profile.avgRating, 4.5);
    });
  });

  group('AnalyticsSummary', () {
    test('computes today revenue, orders, avg value, top items', () {
      final now = DateTime.now();
      final today = DateTime(now.year, now.month, now.day);
      final orders = [
        MerchantOrder.fromJson({
          'id': 'a',
          'status': 'DELIVERED',
          'merchant_status': 'READY',
          'created_at': today.toIso8601String(),
          'total_amount': 60000,
          'items': [
            {'item_name': 'Nasi', 'quantity': 2},
            {'item_name': 'Ayam', 'quantity': 1},
          ],
        }),
        MerchantOrder.fromJson({
          'id': 'b',
          'status': 'CREATED',
          'merchant_status': 'WAITING',
          'created_at': today.toIso8601String(),
          'total_amount': 30000,
          'items': [
            {'item_name': 'Nasi', 'quantity': 1},
          ],
        }),
        MerchantOrder.fromJson({
          'id': 'c',
          'status': 'CANCELLED',
          'merchant_status': 'CANCELLED',
          'created_at': today.toIso8601String(),
          'total_amount': 99999,
        }),
      ];

      final summary = AnalyticsSummary.compute(orders, 'today');

      expect(summary.totalOrders, 2);
      expect(summary.totalRevenue, 90000);
      expect(summary.avgOrderValue, 45000);
      expect(summary.topItems.first.name, 'Nasi');
      expect(summary.topItems.first.quantity, 3);
      expect(summary.dailyRevenue.length, 1);
      expect(summary.dailyRevenue.first.amount, 90000);
    });
  });
}