import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:latlong2/latlong.dart';
import 'package:uuid/uuid.dart';

import '../../config/router.dart';
import '../../models/ride_order.dart';
import '../../providers/ride_provider.dart';
import '../../providers/wallet_provider.dart';
import '../../services/location_service.dart';
import '../../services/ride_service.dart';

class RideBookingScreen extends ConsumerStatefulWidget {
  const RideBookingScreen({super.key});

  @override
  ConsumerState<RideBookingScreen> createState() => _RideBookingScreenState();
}

class _RideBookingScreenState extends ConsumerState<RideBookingScreen> {
  /// Fallback koordinat pickup (Jakarta) saat GPS/izin tidak tersedia.
  static const LatLng _defaultPickup = LatLng(-6.2088, 106.8456);

  // TODO: ganti dengan hasil geocoding/search saat fitur pilih lokasi aktif.
  final LatLng _dropoffLocation = const LatLng(-6.2950, 106.8638);
  final MapController _mapController = MapController();

  LatLng _pickupLocation = _defaultPickup;
  bool _hasRealPickup = false;
  String? _locationNote;

  // TODO: ganti dengan hasil geocoding/search saat fitur pilih lokasi aktif.
  final TextEditingController _pickupController =
      TextEditingController(text: 'Jl. Sudirman 99');
  final TextEditingController _dropoffController =
      TextEditingController(text: 'Jl. Gatot Subroto');
  final TextEditingController _voucherController = TextEditingController();

  String _paymentMethod = 'WALLET';
  bool _isBooking = false;

  /// Debt Gate Universal: booking diblokir sampai banner dibersihkan
  /// (mis. setelah lunas lewat top-up).
  bool _overdueDebt = false;
  int _fareEstimate = 0;
  double _distanceKm = 0;

  @override
  void initState() {
    super.initState();
    _recomputeFare();
    _loadCurrentLocation();
  }

  @override
  void dispose() {
    _pickupController.dispose();
    _dropoffController.dispose();
    _voucherController.dispose();
    super.dispose();
  }

  void _recomputeFare() {
    _distanceKm = RideMockSimulator.distanceKm(_pickupLocation, _dropoffLocation);
    _fareEstimate = estimateFare(pickup: _pickupLocation, dropoff: _dropoffLocation);
  }

  /// Ambil lokasi user real (geolocator). Jika izin ditolak / layanan mati,
  /// pertahankan koordinat default + tampilkan catatan agar user tahu.
  Future<void> _loadCurrentLocation() async {
    final result = await const LocationService().getCurrentLocation();
    if (!mounted) return;

    if (result.hasPosition) {
      setState(() {
        _pickupLocation = LatLng(
          result.position!.latitude,
          result.position!.longitude,
        );
        _hasRealPickup = true;
        _locationNote = null;
        _recomputeFare();
      });
      _mapController.move(_pickupLocation, 14);
      return;
    }

    setState(() {
      _locationNote = switch (result.status) {
        LocationPermissionStatus.serviceDisabled =>
          'Layanan lokasi mati — memakai titik default.',
        LocationPermissionStatus.denied =>
          'Izin lokasi ditolak — memakai titik default.',
        LocationPermissionStatus.deniedForever =>
          'Izin lokasi dinonaktifkan permanen — memakai titik default.',
        LocationPermissionStatus.unavailable =>
          'Lokasi tidak terbaca — memakai titik default.',
        LocationPermissionStatus.granted => null,
      };
    });
  }

  Future<void> _bookRide() async {
    if (_isBooking || _overdueDebt) return;
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
              voucherCode: _voucherController.text,
            );
      } on DioException catch (e) {
        // Debt Gate Universal: customer dengan hutang ditolak backend.
        // Tampilkan banner blocking alih-alih pesan generik.
        if (isOverdueDebtError(e)) {
          if (mounted) setState(() => _overdueDebt = true);
          _showSnack('Booking ditolak: Anda memiliki hutang. Lunasi lewat Wallet.');
          return;
        }
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

  String _formatRupiah(int amount) {
    final digits = amount.toString();
    final buffer = StringBuffer();
    for (var i = 0; i < digits.length; i++) {
      if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
      buffer.write(digits[i]);
    }
    return 'Rp ${buffer.toString()}';
  }

  Widget _overdueDebtBanner() {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.red.shade50,
        border: Border.all(color: Colors.red.shade300),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(Icons.warning_amber_rounded, color: Colors.red.shade700),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Anda memiliki hutang (overdue). Booking diblokir sampai '
              'tagihan dilunasi lewat Wallet.',
              style: TextStyle(color: Colors.red.shade900),
            ),
          ),
          TextButton(
            onPressed: () => setState(() => _overdueDebt = false),
            child: const Text('Coba lagi'),
          ),
        ],
      ),
    );
  }

  Widget _farePreview() {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.grey.shade100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          const Icon(Icons.payments_outlined),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Estimasi ongkos: ${_formatRupiah(_fareEstimate)}\n'
              'Jarak ± ${_distanceKm.toStringAsFixed(1)} km',
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    // TD-196 (S122): proactive debt gate. Fetch /wallets/me on mount via
    // myWalletProvider (auto-load) lalu tampilkan banner bila overdue_debt > 0.
    // Reactive catch (isOverdueDebtError 422) tetap sebagai fallback.
    ref.listen<WalletBalanceState>(myWalletProvider, (prev, next) {
      final debt = next.wallet?.overdueDebt ?? 0;
      if (debt > 0 && !_overdueDebt) {
        setState(() => _overdueDebt = true);
      }
    });
    return Scaffold(
      appBar: AppBar(
        title: const Text('Book Ride'),
        actions: [
          IconButton(
            icon: const Icon(Icons.account_balance_wallet_outlined),
            tooltip: 'Wallet',
            onPressed: () => Navigator.pushNamed(
              context,
              AppRoutes.walletBalance,
            ),
          ),
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
                  if (_overdueDebt) _overdueDebtBanner(),
                  TextField(
                    controller: _pickupController,
                    decoration: InputDecoration(
                      labelText: 'Pickup',
                      prefixIcon: const Icon(Icons.my_location),
                      helperText: _hasRealPickup
                          ? 'Titik jemput dari lokasi Anda'
                          : _locationNote,
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
                  _farePreview(),
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
                  TextField(
                    controller: _voucherController,
                    decoration: const InputDecoration(
                      labelText: 'Kode voucher (opsional)',
                      prefixIcon: Icon(Icons.local_offer_outlined),
                    ),
                  ),
                  const SizedBox(height: 12),
                  ElevatedButton(
                    onPressed: (_isBooking || _overdueDebt) ? null : _bookRide,
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
