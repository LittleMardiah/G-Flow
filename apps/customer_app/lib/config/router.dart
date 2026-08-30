import 'package:flutter/material.dart';

import '../screens/auth/login_screen.dart';
import '../screens/auth/register_screen.dart';
import '../screens/food/food_cart_screen.dart';
import '../screens/food/food_catalog_screen.dart';
import '../screens/food/food_checkout_screen.dart';
import '../screens/food/food_history_screen.dart';
import '../screens/food/food_tracking_screen.dart';
import '../screens/food/merchant_detail_screen.dart';
import '../screens/ride/ride_booking_screen.dart';
import '../screens/ride/ride_tracking_screen.dart';
import '../screens/send/send_history_screen.dart';
import '../screens/send/send_package_form_screen.dart';
import '../screens/send/send_tracking_screen.dart';

/// Konstanta nama route. Parameter order (misal orderId) diteruskan via
/// `arguments:` pada Navigator.pushNamed (bukan path parameter).
class AppRoutes {
  AppRoutes._();

  static const String login = '/login';
  static const String register = '/register';
  static const String home = '/home';

  // Ride
  static const String rideTracking = '/ride-tracking';

  // Food
  static const String foodCatalog = '/food-catalog';
  static const String foodMerchantDetail = '/food-merchant-detail';
  static const String foodCart = '/food-cart';
  static const String foodCheckout = '/food-checkout';
  static const String foodTracking = '/food-tracking';
  static const String foodHistory = '/food-history';

  // Send
  static const String sendForm = '/send-form';
  static const String sendTracking = '/send-tracking';
  static const String sendHistory = '/send-history';
}

/// Daftar route terpusat pengganti map inline di main.dart agar mudah
/// di-share dan di-test.
class AppRouter {
  AppRouter._();

  static Map<String, WidgetBuilder> get routes => {
        AppRoutes.login: (_) => const LoginScreen(),
        AppRoutes.register: (_) => const RegisterScreen(),
        AppRoutes.home: (_) => const RideBookingScreen(),
        AppRoutes.rideTracking: (_) => const RideTrackingScreen(),

        AppRoutes.foodCatalog: (_) => const FoodCatalogScreen(),
        AppRoutes.foodMerchantDetail: (_) => const MerchantDetailScreen(),
        AppRoutes.foodCart: (_) => const FoodCartScreen(),
        AppRoutes.foodCheckout: (_) => const FoodCheckoutScreen(),
        AppRoutes.foodTracking: (_) => const FoodTrackingScreen(),
        AppRoutes.foodHistory: (_) => const FoodHistoryScreen(),

        AppRoutes.sendForm: (_) => const SendPackageFormScreen(),
        AppRoutes.sendTracking: (_) => const SendTrackingScreen(),
        AppRoutes.sendHistory: (_) => const SendHistoryScreen(),
      };
}
