import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:latlong2/latlong.dart';
import 'package:uuid/uuid.dart';

import '../../config/router.dart';
import '../../models/ride_order.dart';
import '../../providers/ride_provider.dart';
import '../../services/ride_service.dart';

class RideBookingScreen extends ConsumerStatefulWidget {
  const RideBookingScreen({super.key});

  @override
  ConsumerState<RideBookingScreen> createState() => _RideBookingScreenState();
}

class _RideBookingScreenState extends ConsumerState<RideBookingScreen> {
  final LatLng _pickupLocation = const LatLng(-6.2088, 106.8456);
  // TODO: ganti dengan hasil geocoding/search saat fitur pilih lokasi aktif.
  final LatLng _dropoffLocation = const LatLng(-6.2950, 106.8638);
  final MapController _mapController = MapController();

  // TODO: ganti dengan geolocator (lokasi user real) & search dropoff.
  final TextEditingController _pickupController =
      TextEditingController(text: 'Jl. Sudirman 99');
  final TextEditingController _dropoffController =
      TextEditingController(text: 'Jl. Gatot Subroto');

  String _paymentMethod = 'WALLET';
  bool _isBooking = false;

  @override
  void dispose() {
    _pickupController.dispose();
    _dropoffController.dispose();
    super.dispose();
  }

  Future<void> _bookRide() async {
    if (_isBooking) return;
    if (_pickupController.text.trim().isEmpty || _dropoffController.text.trim().isEmpty) {
      _showSnack('Pickup dan dropoff wajib diisi');
      return;
    }

    setState(() => _isBooking = true);
    try {
      BookRideResult result;
      try {
        result = await ref.read(rideServiceProvider).bookRide(
              pickupLat: _pickupLocation.latitude,
              pickupLng: _pickupLocation.longitude,
              dropoffLat: _dropoffLocation.latitude,
              dropoffLng: _dropoffLocation.longitude,
              paymentMethod: _paymentMethod,
              idempotencyKey: const Uuid().v4(),
            );
      } on DioException {
        // TODO: hapus fallback mock saat backend stabil diakses dari device.
        // API CONTRACT 7.1 (POST /rides/book) sudah ada di backend, tapi
        // baseUrl masih IP hotspot — jika tidak terjangkau, lanjut mode demo.
        if (!kUseMockRideData) rethrow;
        result = RideMockSimulator.book(
          pickupLat: _pickupLocation.latitude,
          pickupLng: _pickupLocation.longitude,
          dropoffLat: _dropoffLocation.latitude,
          dropoffLng: _dropoffLocation.longitude,
          paymentMethod: _paymentMethod,
        );
        _showSnack('Mode demo: backend tidak terjangkau, data disimulasikan');
      }

      if (!mounted) return;
      Navigator.pushNamed(
        context,
        AppRoutes.rideTracking,
        arguments: RideTrackingArgs(
          orderId: result.orderId,
          pickupLat: _pickupLocation.latitude,
          pickupLng: _pickupLocation.longitude,
          dropoffLat: _dropoffLocation.latitude,
          dropoffLng: _dropoffLocation.longitude,
          pickupAddress: _pickupController.text.trim(),
          dropoffAddress: _dropoffController.text.trim(),
          distanceKm: result.distanceKm,
          estimatedFare: result.estimatedFare,
          paymentMethod: result.paymentMethod,
          isMock: result.isMock,
        ),
      );
    } catch (_) {
      _showSnack('Gagal membuat booking. Periksa koneksi lalu coba lagi.');
    } finally {
      if (mounted) setState(() => _isBooking = false);
    }
  }

  void _showSnack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Book Ride'),
        actions: [
          IconButton(
            icon: const Icon(Icons.history),
            tooltip: 'Riwayat Perjalanan',
            onPressed: () => Navigator.pushNamed(context, AppRoutes.rideHistory),
          ),
        ],
      ),
      body: Column(
        children: [
          // Peta (2/3 layar)
          Expanded(
            flex: 2,
            child: FlutterMap(
              mapController: _mapController,
              options: MapOptions(
                initialCenter: _pickupLocation,
                initialZoom: 12,
              ),
              children: [
                TileLayer(
                  urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                  userAgentPackageName: 'com.gflow.customer_app',
                ),
                MarkerLayer(
                  markers: [
                    Marker(
                      point: _pickupLocation,
                      width: 40,
                      height: 40,
                      child: const Icon(Icons.location_pin, color: Colors.red, size: 40),
                    ),
                    Marker(
                      point: _dropoffLocation,
                      width: 40,
                      height: 40,
                      child: const Icon(Icons.place, color: Colors.green, size: 36),
                    ),
                  ],
                ),
              ],
            ),
          ),
          // Form bawah (1/3 layar) → BISA DI-SCROLL
          Expanded(
            flex: 1,
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  TextField(
                    controller: _pickupController,
                    decoration: const InputDecoration(
                      labelText: 'Pickup',
                      prefixIcon: Icon(Icons.my_location),
                    ),
                  ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _dropoffController,
                    decoration: const InputDecoration(
                      labelText: 'Dropoff',
                      prefixIcon: Icon(Icons.place),
                    ),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                    children: [
                      ChoiceChip(
                        label: const Text('WALLET'),
                        avatar: const Icon(Icons.wallet, size: 16),
                        selected: _paymentMethod == 'WALLET',
                        onSelected: (_) => setState(() => _paymentMethod = 'WALLET'),
                      ),
                      ChoiceChip(
                        label: const Text('CASH'),
                        avatar: const Icon(Icons.money, size: 16),
                        selected: _paymentMethod == 'CASH',
                        onSelected: (_) => setState(() => _paymentMethod = 'CASH'),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  ElevatedButton(
                    onPressed: _isBooking ? null : _bookRide,
                    child: Text(_isBooking ? 'Memproses…' : 'Book Ride'),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}